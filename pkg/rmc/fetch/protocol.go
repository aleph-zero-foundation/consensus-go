package fetch

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
)

const (
	SendData byte = iota
	AcceptData
	SendProof
	AcceptProof
)

type protocol struct {
	dag      gomel.Dag
	rs       gomel.RandomSource
	pid      uint16
	nProc    int
	requests chan gomel.Preunit
	state    *rmc.State
	dialer   network.Dialer
	listener network.Listener
	timeout  time.Duration
	log      zerolog.Logger
}

func newProtocol(pid uint16, dag gomel.Dag, rs gomel.RandomSource, requests chan gomel.Preunit, state *rmc.State, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) *protocol {
	return &protocol{
		rs:       rs,
		pid:      pid,
		dag:      dag,
		nProc:    dag.NProc(),
		requests: requests,
		state:    state,
		dialer:   dialer,
		listener: listener,
		timeout:  timeout,
		log:      log,
	}
}

func (p *protocol) In() {
	conn, err := p.listener.Listen(p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	_, id, _, err := rmc.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.in.greeting").Msg(err.Error())
		return
	}
	rmc.SendStatus(conn, p.state.Status(id))
	switch p.state.Status(id) {
	case rmc.Data:
		p.state.SendData(id, p.state.Data(id), conn)
	case rmc.Signed:
		p.state.SendSignature(id, conn)
	case rmc.Finished:
		p.state.SendFinished(id, conn)
	}
}

func (p *protocol) Out() {
	pu, ok := <-p.requests
	for {
		time.Sleep(1 * time.Second)
		ok := false
		p.dag.AddUnit(pu, p.rs, func(_ gomel.Preunit, u gomel.Unit, err error) {
			if u != nil {
				ok = true
			}
		})
		if ok {
			return
		}
	}

	return
	if !ok {
		return
	}
	pid := uint16(pu.Creator())
	conn, err := p.dialer.Dial(pid)
	if err != nil {
		p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	/*err = rmc.Greet(conn, r.pid, r.id, r.msgType)
	if err != nil {
		p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
		return
	}*/
	status, id, err := rmc.AcceptStatus(conn)
	if err != nil {
		p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
		return
	}

	switch status {
	case rmc.Unknown:
		return
	case rmc.Data:
		p.state.AcceptData(id, pid, conn)
	case rmc.Signed:
		p.state.AcceptSignature(id, pid, conn)
	case rmc.Finished:
		p.state.AcceptFinished(id, pid, conn)
	}
}
