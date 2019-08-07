package rmc

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

const (
	SendData byte = iota
	AcceptData
	SendProof
	AcceptProof
)

type Request struct {
	msgType byte
	id      uint64
	pid     uint16
	data    []byte
}

func NewRequest(id uint64, pid uint16, data []byte, msgType byte) Request {
	return Request{
		msgType: msgType,
		id:      id,
		pid:     pid,
		data:    data,
	}
}

type protocol struct {
	pid      uint16
	nProc    int
	requests chan Request
	state    *RMC
	accepted chan []byte
	dialer   network.Dialer
	listener network.Listener
	timeout  time.Duration
	log      zerolog.Logger
}

func newProtocol(pid uint16, nProc int, requests chan Request, state *RMC, accepted chan []byte, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) *protocol {
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
		p.log.Error().Str("where", "multicast.In.Listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	pid, id, msgType, err := AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "multicast.In.Listen").Msg(err.Error())
		return
	}
	switch msgType {
	case SendData:
		_, err := p.state.AcceptData(id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "multicast.In.Listen").Msg(err.Error())
			return
		}
		err = p.state.SendSignature(id, conn)
		if err != nil {
			p.log.Error().Str("where", "multicast.In.Listen").Msg(err.Error())
			return
		}
	case SendProof:
		err := p.state.AcceptProof(id, conn)
		if err != nil {
			p.log.Error().Str("where", "multicast.In.Listen").Msg(err.Error())
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
		p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	err = Greet(conn, r.pid, r.id, r.msgType)
	if err != nil {
		p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
		return
	}

	switch r.msgType {
	case SendData:
		err := p.state.SendData(r.id, r.data, conn)
		if err != nil {
			p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
			return
		}
		finished, err := p.state.AcceptSignature(r.id, r.pid, conn)
		if err != nil {
			p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
			return
		}
		if finished {
			for i := 0; i < p.nProc; i++ {
				if uint16(i) == p.pid {
					continue
				}
				p.requests <- NewRequest(r.id, uint16(i), r.data, SendProof)
			}
		}
	case SendProof:
		err := p.state.SendProof(r.id, conn)
		if err != nil {
			p.log.Error().Str("where", "multicast.Out.Dial").Msg(err.Error())
			return
		}
	}
}
