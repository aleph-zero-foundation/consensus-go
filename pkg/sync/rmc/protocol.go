package rmc

import (
	"math/rand"

	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	rmcbox "gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

func (p *server) in() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		p.log.Error().Str("where", "rmc.in.Listen").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)

	pid, id, msgType, err := rmcbox.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "rmc.in.AcceptGreeting").Msg(err.Error())
		return
	}
	log := p.log.With().Uint16(logging.PID, pid).Uint64(logging.ISID, id).Logger()
	conn.SetLogger(log)
	log.Info().Msg(logging.SyncStarted)

	var toAdd gomel.Preunit
	switch msgType {
	case sendData:
		data, err := p.state.AcceptData(id, pid, conn)
		if err != nil {
			log.Error().Str("where", "rmc.in.AcceptData").Msg(err.Error())
			return
		}
		pu, err := encoding.DecodePreunit(data)
		if err != nil {
			log.Error().Str("where", "rmc.in.DecodePreunit").Msg(err.Error())
			return
		}
		if id != preunitID(pu, p.dag.NProc()) {
			log.Error().Str("what", "wrong preunit id").Msg(err.Error())
			return
		}

		err = p.state.SendSignature(id, conn)
		if err != nil {
			log.Error().Str("where", "rmc.in.SendSignature").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			log.Error().Str("where", "rmc.in.Flush3").Msg(err.Error())
			return
		}

	case sendFinished:
		_, err := p.state.AcceptFinished(id, pid, conn)
		if err != nil {
			log.Error().Str("where", "rmc.in.AcceptFinished2").Msg(err.Error())
			return
		}
		pu, err := encoding.DecodePreunit(p.state.Data(id))
		if err != nil {
			log.Error().Str("where", "rmc.in.DecodePreunit3").Msg(err.Error())
			return
		}
		toAdd = pu
	}
	if toAdd != nil {
		if !add.Unit(p.adder, toAdd, p.fallback, "rmc.in.Predecessor", log) {
			return
		}
	}
	log.Info().Msg(logging.SyncCompleted)
}

func (p *server) out(pid uint16) {
	r, ok := <-p.requests[pid]
	if !ok {
		return
	}
	conn, err := p.netserv.Dial(pid, p.timeout)
	if err != nil {
		p.log.Error().Str("where", "rmc.out.Dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	err = rmcbox.Greet(conn, p.pid, r.id, r.msgType)
	if err != nil {
		p.log.Error().Str("where", "rmc.out.Greet").Msg(err.Error())
		return
	}
	log := p.log.With().Uint16(logging.PID, pid).Uint64(logging.OSID, r.id).Logger()
	conn.SetLogger(log)
	log.Info().Msg(logging.SyncStarted)

	switch r.msgType {
	case sendData:
		err := p.state.SendData(r.id, r.data, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.out.SendData").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			p.log.Error().Str("where", "rmc.out.Flush").Msg(err.Error())
			return
		}

		finished, err := p.state.AcceptSignature(r.id, pid, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.out.AcceptSignature").Msg(err.Error())
			return
		}
		if finished {
			for _, i := range rand.Perm(int(p.dag.NProc())) {
				if i == int(p.pid) {
					continue
				}
				p.requests[i] <- newRequest(r.id, r.data, sendFinished)
			}
			p.canMulticast.Unlock()
		}

	case sendFinished:
		err := p.state.SendFinished(r.id, conn)
		if err != nil {
			p.log.Error().Str("where", "rmc.out.SendFinished").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			p.log.Error().Str("where", "rmc.out.Flush3").Msg(err.Error())
			return
		}
	}
	log.Info().Msg(logging.SyncCompleted)
}
