package multicast

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

func (p *server) in() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		p.log.Error().Str("where", "multicast.in.Listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	decoder := custom.NewDecoder(conn)
	preunit, err := decoder.DecodePreunit()
	if err != nil {
		p.log.Error().Str("where", "multicast.in.Decode").Msg(err.Error())
		return
	}
	err = add.Unit(p.dag, p.randomSource, preunit, p.fallback, p.log)
	if err != nil {
		p.log.Error().Str("where", "multicast.in.AddUnit").Msg(err.Error())
		return
	}
	p.log.Info().Uint16(logging.Creator, preunit.Creator()).Msg(logging.AddedBCUnit)
}

func (p *server) out(pid uint16) {
	r, ok := <-p.requests[pid]
	if !ok {
		return
	}
	conn, err := p.netserv.Dial(pid, p.timeout)
	if err != nil {
		p.log.Error().Str("where", "multicast.out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	_, err = conn.Write(r.encUnit)
	if err != nil {
		p.log.Error().Str("where", "multicast.out.SendUnit").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		p.log.Error().Str("where", "multicast.out.Flush").Msg(err.Error())
		return
	}
	p.log.Info().Int(logging.Height, r.height).Uint16(logging.PID, pid).Msg(logging.UnitBroadcasted)
}
