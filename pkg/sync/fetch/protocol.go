package fetch

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

func (p *server) in() {
	conn, err := p.netserv.Listen(p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	pid, sid, err := handshake.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.in.greeting").Msg(err.Error())
		return
	}
	if pid >= p.dag.NProc() {
		p.log.Warn().Uint16(logging.PID, pid).Msg("Called by a stranger")
		return
	}
	log := p.log.With().Uint16(logging.PID, pid).Uint32(logging.ISID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)
	conn.SetLogger(log)
	log.Debug().Msg(logging.GetRequests)
	hashes, err := receiveRequests(conn)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.receiveRequests").Msg(err.Error())
		return
	}
	units, err := getUnits(p.dag, hashes)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.getUnits").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.SendUnits)
	err = encoding.SendUnits(units, conn)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.in.sendUnits").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Sent, len(units)).Msg(logging.SyncCompleted)
}

func (p *server) out() {
	r, ok := <-p.requests
	if !ok {
		return
	}
	remotePid := r.pid
	conn, err := p.netserv.Dial(remotePid, p.timeout)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		p.log.Error().Str("where", "fetchProtocol.out.greeting").Msg(err.Error())
		return
	}
	log := p.log.With().Uint16(logging.PID, remotePid).Uint32(logging.OSID, sid).Logger()
	log.Info().Msg(logging.SyncStarted)
	conn.SetLogger(log)
	log.Debug().Msg(logging.SendRequests)
	err = sendRequests(conn, r.hashes)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.out.sendRequests").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.GetPreunits)
	units, nReceived, err := encoding.GetPreunits(conn)
	if err != nil {
		log.Error().Str("where", "fetchProtocol.out.receivePreunits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(units)).Msg(logging.ReceivedPreunits)
	err = add.Units(p.dag, p.adder, units, p.fallback, "fetch.out.addUnits", log)
	if err != nil {
		log.Error().Str("where", "fetch.out.addUnits").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Recv, nReceived).Msg(logging.SyncCompleted)
}
