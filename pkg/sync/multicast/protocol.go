package multicast

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

const (
	// Some magic numbers for multicast. All below are ratios, they get multiplied with nProc.
	requestsSize = 10
	mcOutWPSize  = 4
	mcInWPSize   = 2
)

type protocol struct {
	pid          uint16
	dag          gomel.Dag
	randomSource gomel.RandomSource
	requests     <-chan request
	netserv      network.Server
	callback     gomel.Callback
	timeout      time.Duration
	fallback     sync.Fallback
	log          zerolog.Logger
}

func newProtocol(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, requests <-chan request, netserv network.Server, callback gomel.Callback, timeout time.Duration, fallback sync.Fallback, log zerolog.Logger) sync.Protocol {

	logSuccess := func(_ gomel.Preunit, added gomel.Unit, err error) {
		if err == nil {
			log.Info().Int(logging.Creator, added.Creator()).Int(logging.Height, added.Height()).Msg(logging.AddedBCUnit)
		}
	}

	return &protocol{
		pid:          pid,
		dag:          dag,
		randomSource: randomSource,
		requests:     requests,
		netserv:      netserv,
		callback:     gomel.MergeCallbacks(callback, logSuccess),
		timeout:      timeout,
		fallback:     fallback,
		log:          log,
	}
}

func (p *protocol) In() {
	conn, err := p.netserv.Listen(p.timeout)
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
	err = add.Unit(p.dag, p.randomSource, preunit, p.callback, p.fallback, p.log)
}

func (p *protocol) Out() {
	r, ok := <-p.requests
	if !ok {
		return
	}
	conn, err := p.netserv.Dial(r.pid)
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
