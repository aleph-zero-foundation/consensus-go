package multicast

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
)

type protocol struct {
	pid      uint16
	nProc    int
	requests chan Request
	state    *rmc.State
	accepted chan []byte
	dialer   network.Dialer
	listener network.Listener
	timeout  time.Duration
	log      zerolog.Logger
	inUse    []*rmc.Mutex
}

func newProtocol(pid uint16, nProc int, requests chan Request, state *rmc.State, accepted chan []byte, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) *protocol {
	return &protocol{
		pid:      pid,
		nProc:    nProc,
		requests: requests,
		state:    state,
		accepted: accepted,
		dialer:   dialer,
		listener: listener,
		timeout:  timeout,
		log:      log,
	}
}

func (p *protocol) In() {
	conn, err := p.listener.Listen(p.timeout)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	pid, id, msgType, err := rmc.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
		return
	}
	switch msgType {
	case SendData:
		//fmt.Println("JESTEM ", p.pid, " <= DANE ", pid)
		_, err := p.state.AcceptData(id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		err = p.state.SendSignature(id, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		conn.Flush()
	case SendFinished:
		_, err := p.state.AcceptFinished(id, pid, conn)
		//fmt.Println("JESTEM", p.pid, "<= DOWOD", id, "OD ", pid)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.In.Listen").Msg(err.Error())
			return
		}
		p.accepted <- p.state.Data(id)
	}
}

func (p *protocol) Out() {
	r, ok := <-p.requests
	if !ok {
		return
	}
	conn, err := p.dialer.Dial(r.pid)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	err = rmc.Greet(conn, p.pid, r.id, r.msgType)
	if err != nil {
		p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
		return
	}

	switch r.msgType {
	case SendData:
		//fmt.Println("JESTEM ", p.pid, "DANE => ", r.pid)
		err := p.state.SendData(r.id, r.data, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
			return
		}
		conn.Flush()

		statusBefore := p.state.Status(r.id)
		finished, err := p.state.AcceptSignature(r.id, r.pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
			return
		}
		if finished && statusBefore != rmc.Finished {
			for i := 0; i < p.nProc; i++ {
				if uint16(i) == p.pid {
					continue
				}
				p.requests <- NewRequest(r.id, uint16(i), r.data, SendFinished)
			}
		}
	case SendFinished:
		err := p.state.SendFinished(r.id, conn)
		//fmt.Println("JESTEM ", p.pid, "DOWOD => ", r.pid)
		if err != nil {
			p.log.Error().Str("where", "rmc.multicast.Out.Dial").Msg(err.Error())
			return
		}
		conn.Flush()
	}
}
