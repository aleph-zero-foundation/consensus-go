package fetch

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
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
	if pid >= uint16(len(p.syncIds)) {
		p.log.Warn().Uint16(logging.PID, pid).Msg("Called by a stranger")
		return
	}
	log := p.log.With().Uint16(logging.PID, pid).Uint32(logging.ISID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)
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
	log.Debug().Int(logging.Sent, len(units)).Msg(logging.SendUnits)
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
	log := p.log.With().Uint16(logging.PID, remotePid).Uint32(logging.OSID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)

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
	log.Debug().Msg(logging.GetUnits)
	units, err := encoding.ReadChunk(conn)
	nReceived := len(units)
	if err != nil {
		log.Error().Str("where", "fetch.out.receivePreunits").Msg(err.Error())
		return
	}
	logging.AddingErrors(p.orderer.AddPreunits(remotePid, units...), log)
	log.Info().Int(logging.Recv, nReceived).Msg(logging.SyncCompleted)
}
