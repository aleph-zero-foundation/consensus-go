package multicast

import (
	"bytes"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

const (
	// Some magic numbers for multicast. All below are ratios, they get multiplied with nProc.
	requestsSize = 10
	mcOutWPSize  = 4
	mcInWPSize   = 2
)

//request represents a request to send the encoded unit to the committee member indicated by pid.
type request struct {
	encUnit []byte
	height  int
	pid     uint16
}

type protocol struct {
	pid          uint16
	dag          gomel.Dag
	randomSource gomel.RandomSource
	requests     chan request
	dialer       network.Dialer
	listener     network.Listener
	timeout      time.Duration
	log          zerolog.Logger
}

func newProtocol(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) *protocol {
	return &protocol{
		pid:          pid,
		dag:          dag,
		randomSource: randomSource,
		requests:     make(chan request, requestsSize*dag.NProc()),
		dialer:       dialer,
		listener:     listener,
		timeout:      timeout,
		log:          log,
	}
}

//Request encodes the given unit and pushes to the internal channel requests to send that unit to every committee member other than one's own.
func (p *protocol) request(unit gomel.Unit) error {
	buffer := &bytes.Buffer{}
	encoder := custom.NewEncoder(buffer)
	err := encoder.EncodeUnit(unit)
	if err != nil {
		return err
	}
	encUnit := buffer.Bytes()[:]
	perm := rand.Perm(p.dag.NProc())
	for _, i := range perm {
		if i == int(p.pid) {
			continue
		}
		p.requests <- request{encUnit, unit.Height(), uint16(i)}
	}
	return nil
}

func (p *protocol) In() {
	conn, err := p.listener.Listen(p.timeout)
	if err != nil {
		p.log.Error().Str("where", "multicast.In.Listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	decoder := custom.NewDecoder(conn)
	preunit, err := decoder.DecodePreunit()
	if err != nil {
		p.log.Error().Str("where", "multicast.In.Decode").Msg(err.Error())
		return
	}
	var wg sync.WaitGroup
	wg.Add(1)
	p.dag.AddUnit(preunit, p.randomSource, func(pu gomel.Preunit, added gomel.Unit, err error) {
		defer wg.Done()
		if err != nil {
			switch e := err.(type) {
			case *gomel.DuplicateUnit:
				p.log.Info().Int(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
			case *gomel.UnknownParents:
				p.log.Info().Int(logging.Creator, pu.Creator()).Int(logging.Size, e.Amount).Msg(logging.UnknownParents)
			default:
				p.log.Error().Str("where", "multicast.In.AddUnit").Msg(err.Error())
			}
			return
		}
		p.log.Info().Int(logging.Creator, added.Creator()).Int(logging.Height, added.Height()).Msg(logging.AddedBCUnit)
	})
	wg.Wait()
}

func (p *protocol) Out() {
	r, ok := <-p.requests
	if !ok {
		return
	}
	conn, err := p.dialer.Dial(r.pid)
	if err != nil {
		p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	_, err = conn.Write(r.encUnit)
	if err != nil {
		p.log.Error().Str("where", "multicast.Out.SendUnit").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		p.log.Error().Str("where", "multicast.Out.Flush").Msg(err.Error())
		return
	}
	p.log.Info().Int(logging.Height, r.height).Uint16(logging.PID, r.pid).Msg(logging.UnitBroadcasted)
}
