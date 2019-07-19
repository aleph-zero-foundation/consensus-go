package multicast

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

type protocol struct {
	pid          uint16
	dag          gomel.Dag
	randomSource gomel.RandomSource
	mcRequests   <-chan MCRequest
	dialer       network.Dialer
	listener     network.Listener
	timeout      time.Duration
	log          zerolog.Logger
}

// NewProtocol returns a new multicast protocol.
func NewProtocol(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, dialer network.Dialer, listener network.Listener, timeout time.Duration, mcRequests <-chan MCRequest, log zerolog.Logger) gsync.Protocol {
	return &protocol{
		pid:          pid,
		dag:          dag,
		randomSource: randomSource,
		mcRequests:   mcRequests,
		dialer:       dialer,
		listener:     listener,
		timeout:      timeout,
		log:          log,
	}
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
	r, ok := <-p.mcRequests
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
	p.log.Info().Int(logging.Height, r.height).Msg(logging.UnitBroadcasted)
}
