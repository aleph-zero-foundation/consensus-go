package multicast

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

func (p *server) In() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		p.log.Error().Str("where", "multicast.in.listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	preunit, err := encoding.ReceivePreunit(conn)
	if err != nil {
		p.log.Error().Str("where", "multicast.in.decode").Msg(err.Error())
		return
	}
	if add.Unit(p.dag, p.adder, preunit, "multicast.in", p.log) {
		p.log.Info().Uint16(logging.Creator, preunit.Creator()).Msg(logging.AddedBCUnit)
	}
}

func (p *server) Out(pid uint16) {
	r, ok := <-p.requests[pid]
	if !ok {
		return
	}
	conn, err := p.netserv.Dial(pid, p.timeout)
	if err != nil {
		p.log.Error().Str("where", "multicast.out.dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	_, err = conn.Write(r.encUnit)
	if err != nil {
		p.log.Error().Str("where", "multicast.out.sendUnit").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		p.log.Error().Str("where", "multicast.out.flush").Msg(err.Error())
		return
	}
	p.log.Info().Int(logging.Height, r.height).Uint16(logging.PID, pid).Msg(logging.UnitBroadcasted)
}
