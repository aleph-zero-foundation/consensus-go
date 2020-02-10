package fetch

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

func (p *server) In() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	pid, sid, err := handshake.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "fetch.in.greeting").Msg(err.Error())
		return
	}
	if pid >= p.dag.NProc() {
		p.log.Warn().Uint16(logging.PID, pid).Msg("Called by a stranger")
		return
	}
	log := p.log.With().Uint16(logging.PID, pid).Uint32(logging.ISID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)
	log.Debug().Msg(logging.GetRequests)
	unitIDs, err := receiveRequests(conn)
	if err != nil {
		log.Error().Str("where", "fetch.in.receiveRequests").Msg(err.Error())
		return
	}
	units := getUnits(p.dag, unitIDs)
	if err != nil {
		log.Error().Str("where", "fetch.in.getUnits").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.SendUnits)
	err = encoding.SendChunk(units, conn)
	if err != nil {
		log.Error().Str("where", "fetch.in.sendUnits").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "fetch.in.flush").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Sent, len(units)).Msg(logging.SyncCompleted)
}

func (p *server) Out() {
	r, ok := <-p.requests
	if !ok {
		return
	}
	remotePid := r.Pid
	conn, err := p.netserv.Dial(remotePid, p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		p.log.Error().Str("where", "fetch.out.greeting").Msg(err.Error())
		return
	}
	log := p.log.With().Uint16(logging.PID, remotePid).Uint32(logging.OSID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)
	log.Debug().Int(logging.Size, len(r.UnitIDs)).Msg(logging.SendRequests)
	err = sendRequests(conn, r.UnitIDs)
	if err != nil {
		log.Error().Str("where", "fetch.out.sendRequests").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.GetPreunits)
	units, err := encoding.ReceiveChunk(conn)
	nReceived := len(units)
	if err != nil {
		log.Error().Str("where", "fetch.out.receivePreunits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nReceived).Msg(logging.ReceivedPreunits)
	if add.Chunk(p.adder, units, remotePid, "fetch.out", log) {
		log.Info().Int(logging.Recv, nReceived).Msg(logging.SyncCompleted)
	}
}
