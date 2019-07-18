package fetch

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

type protocol struct {
	pid           uint16
	dag           gomel.Dag
	randomSource  gomel.RandomSource
	reqs          <-chan Request
	dialer        network.Dialer
	listener      network.Listener
	syncIds       []uint32
	timeout       time.Duration
	fallback      gsync.Fallback
	attemptTiming chan<- int
	log           zerolog.Logger
}

// NewProtocol returns a new fetching protocol.
// It will wait on reqs to initiate syncing.
// When adding units fails because of missing parents it will call fallback with the unit containing the unknown parents.
func NewProtocol(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, reqs <-chan Request, dialer network.Dialer, listener network.Listener, timeout time.Duration, fallback gsync.Fallback, attemptTiming chan<- int, log zerolog.Logger) gsync.Protocol {
	nProc := uint16(dialer.Length())
	return &protocol{
		pid:           pid,
		dag:           dag,
		randomSource:  randomSource,
		reqs:          reqs,
		dialer:        dialer,
		listener:      listener,
		syncIds:       make([]uint32, nProc),
		timeout:       timeout,
		fallback:      fallback,
		attemptTiming: attemptTiming,
		log:           log,
	}
}

func (p *protocol) In() {
	conn, err := p.listener.Listen(p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	pid, sid, err := handshake.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.in.greeting").Msg(err.Error())
		return
	}
	if pid >= uint16(p.dialer.Length()) {
		p.log.Warn().Uint16(logging.PID, pid).Msg("Called by a stranger")
		return
	}
	log := p.log.With().Uint16(logging.PID, pid).Uint32(logging.ISID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)
	conn.SetLogger(log)
	log.Debug().Msg(logging.GetRequests)
	hashes, err := receiveRequests(conn)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.receiveRequests").Msg(err.Error())
		return
	}
	units, err := getUnits(p.dag, hashes)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.getUnits").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.SendUnits)
	err = sendUnits(conn, units)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.sendUnits").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Sent, len(units)).Msg(logging.SyncCompleted)
}

func (p *protocol) Out() {
	r, ok := <-p.reqs
	if !ok {
		return
	}
	remotePid := r.Pid
	conn, err := p.dialer.Dial(remotePid)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.greeting").Msg(err.Error())
		return
	}
	log := p.log.With().Int(logging.PID, int(remotePid)).Uint32(logging.OSID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)
	conn.SetLogger(log)
	log.Debug().Msg(logging.SendRequests)
	err = sendRequests(conn, r.Hashes)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.out.sendRequests").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.GetPreunits)
	units, err := receivePreunits(conn, len(r.Hashes))
	if err != nil {
		log.Error().Str("where", "fetchProtocol.out.receivePreunits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(units)).Msg(logging.ReceivedPreunits)
	primeAdded, aggErr := add.Antichain(p.dag, p.randomSource, units, p.fallback, log)
	aggErr = aggErr.Pruned(true)
	if aggErr != nil {
		log.Error().Str("where", "fetchProtocol.out.addAntichain").Msg(err.Error())
		return
	}
	if primeAdded {
		select {
		case p.attemptTiming <- -1:
		default:
		}
	}
	log.Info().Int(logging.Recv, len(units)).Msg(logging.SyncCompleted)
}

func sendRequests(conn network.Connection, hashes []*gomel.Hash) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(hashes)))
	_, err := conn.Write(buf)
	if err != nil {
		return err
	}
	for _, h := range hashes {
		_, err = conn.Write(h[:])
		if err != nil {
			return err
		}
	}
	return conn.Flush()
}

func receiveRequests(conn network.Connection) ([]*gomel.Hash, error) {
	buf := make([]byte, 4)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, err
	}
	result := make([]*gomel.Hash, binary.LittleEndian.Uint32(buf))
	for i := range result {
		result[i] = &gomel.Hash{}
		_, err = io.ReadFull(conn, result[i][:])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func sendUnits(conn network.Connection, units []gomel.Unit) error {
	encoder := custom.NewEncoder(conn)
	for _, u := range units {
		err := encoder.EncodeUnit(u)
		if err != nil {
			return err
		}
	}
	return conn.Flush()
}

func receivePreunits(conn network.Connection, k int) ([]gomel.Preunit, error) {
	var err error
	result := make([]gomel.Preunit, k)
	decoder := custom.NewDecoder(conn)
	for i := range result {
		result[i], err = decoder.DecodePreunit()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func getUnits(dag gomel.Dag, hashes []*gomel.Hash) ([]gomel.Unit, error) {
	units := dag.Get(hashes)
	for i, u := range units {
		if u == nil {
			return nil, fmt.Errorf("received request for unknown hash: %s", hashes[i].Short())
		}
	}
	return units, nil
}
