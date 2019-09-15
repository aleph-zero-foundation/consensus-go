package multicast

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

func (p *server) In() {
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
	err = add.Unit(p.dag, p.randomSource, preunit, p.log)
	if err != nil {
		//TODO FALLBACK
		p.log.Error().Str("where", "multicast.In.AddUnit").Msg(err.Error())
		return
	}
}

func (p *server) Out(pid uint16) {
	r, ok := <-p.requests[pid]
	if !ok {
		return
	}
	conn, err := p.netserv.Dial(pid, p.timeout)
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
	p.log.Info().Int(logging.Height, r.height).Uint16(logging.PID, pid).Msg(logging.UnitBroadcasted)
}
