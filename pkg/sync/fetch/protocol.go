package fetch

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

func (p *server) In() {
	conn, err := p.netserv.Listen()
	if err != nil {
		return
	}
	defer conn.Close()
	pid, sid, err := handshake.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "fetch.in.greeting").Msg(err.Error())
		return
	}
	if pid >= uint16(len(p.syncIds)) {
		p.log.Warn().Uint16(lg.PID, pid).Msg("Called by a stranger")
		return
	}
	log := p.log.With().Uint16(lg.PID, pid).Uint32(lg.ISID, sid).Logger()
	log.Info().Msg(lg.SyncStarted)
	unitIDs, err := receiveRequests(conn)
	if err != nil {
		log.Error().Str("where", "fetch.in.receiveRequests").Msg(err.Error())
		return
	}
	units := p.orderer.UnitsByID(unitIDs...)
	if err != nil {
		log.Error().Str("where", "fetch.in.getUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(lg.Sent, len(units)).Msg(lg.SendUnits)
	err = encoding.WriteChunk(units, conn)
	if err != nil {
		log.Error().Str("where", "fetch.in.sendUnits").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "fetch.in.flush").Msg(err.Error())
		return
	}
	log.Info().Int(lg.Sent, len(units)).Msg(lg.SyncCompleted)
}

func (p *server) Out() {
	r, ok := <-p.requests
	if !ok {
		return
	}
	remotePid := r.Pid
	conn, err := p.netserv.Dial(remotePid)
	if err != nil {
		return
	}
	defer conn.Close()
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	log := p.log.With().Uint16(lg.PID, remotePid).Uint32(lg.OSID, sid).Logger()
	log.Info().Msg(lg.SyncStarted)

	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		log.Error().Str("where", "fetch.out.greeting").Msg(err.Error())
		return
	}
	err = sendRequests(conn, r.UnitIDs)
	if err != nil {
		log.Error().Str("where", "fetch.out.sendRequests").Msg(err.Error())
		return
	}
	log.Debug().Msg(lg.GetUnits)
	units, err := encoding.ReadChunk(conn)
	nReceived := len(units)
	if err != nil {
		log.Error().Str("where", "fetch.out.receivePreunits").Msg(err.Error())
		return
	}
	errs := p.orderer.AddPreunits(remotePid, units...)
	lg.AddingErrors(errs, len(units), log)
	log.Info().Int(lg.Recv, nReceived).Msg(lg.SyncCompleted)
}
