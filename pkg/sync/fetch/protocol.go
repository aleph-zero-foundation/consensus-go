package fetch

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

type protocol struct {
	pid           uint16
	poset         gomel.Poset
	randomSource  gomel.RandomSource
	reqs          <-chan Request
	dialer        network.Dialer
	listener      network.Listener
	syncIds       []uint32
	timeout       time.Duration
	fallback      func(gomel.Preunit)
	attemptTiming chan<- int
	log           zerolog.Logger
}

// NewProtocol returns a new fetching protocol.
// It will wait on reqs to initiate syncing.
// When adding units fails because of missing parents it will call fallback with the unit containing the unknown parents.
func NewProtocol(pid uint16, poset gomel.Poset, randomSource gomel.RandomSource, reqs <-chan Request, dialer network.Dialer, listener network.Listener, timeout time.Duration, fallback func(gomel.Preunit), attemptTiming chan<- int, log zerolog.Logger) gsync.Protocol {
	nProc := uint16(dialer.Length())
	return &protocol{
		pid:           pid,
		poset:         poset,
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
	conn.SetLogger(log)
	//TODO: loggign
	hashes, err := receiveRequests(conn)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.receiveRequests").Msg(err.Error())
		return
	}
	units, err := getUnits(p.poset, hashes)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.getUnits").Msg(err.Error())
		return
	}
	err = sendUnits(conn, units)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.sendUnits").Msg(err.Error())
		return
	}
}

func (p *protocol) Out() {
	r := <-p.reqs
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
	conn.SetLogger(log)
	//TODO: logging
	err = sendRequests(conn, r.Hashes)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.out.sendRequests").Msg(err.Error())
		return
	}
	units, err := receivePreunits(conn, len(r.Hashes))
	if err != nil {
		log.Error().Str("where", "fetchProtocol.out.receivePreunits").Msg(err.Error())
		return
	}
	primeAdded, err := addUnits(p.poset, p.randomSource, units, p.fallback, log)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.out.addUnits").Msg(err.Error())
		return
	}
	if primeAdded {
		select {
		case p.attemptTiming <- -1:
		default:
		}
	}
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

func getUnits(poset gomel.Poset, hashes []*gomel.Hash) ([]gomel.Unit, error) {
	units := poset.Get(hashes)
	for i, u := range units {
		if u == nil {
			return nil, fmt.Errorf("received request for unknown hash: %s", hashes[i].Short())
		}
	}
	return units, nil
}

func addUnits(poset gomel.Poset, randomSource gomel.RandomSource, preunits []gomel.Preunit, fallback func(gomel.Preunit), log zerolog.Logger) (bool, error) {
	var wg sync.WaitGroup
	// TODO: We only report one error, we might want to change it when we deal with Byzantine processes.
	var problem error
	primeAdded := false
	for _, preunit := range preunits {
		wg.Add(1)
		poset.AddUnit(preunit, randomSource, func(pu gomel.Preunit, added gomel.Unit, err error) {
			if err != nil {
				switch e := err.(type) {
				case *gomel.DuplicateUnit:
					log.Info().Int(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
				case *gomel.UnknownParent:
					log.Info().Int(logging.Creator, pu.Creator()).Msg(logging.UnknownParents)
					fallback(pu)
				default:
					problem = err
				}
			} else {
				if gomel.Prime(added) {
					primeAdded = true
				}
			}
			wg.Done()
		})
	}
	wg.Wait()
	return primeAdded, problem
}
