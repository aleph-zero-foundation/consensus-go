package gossip

import (
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

// in handles the incoming connection using info from the dag.
// This version uses simple 2-exchange protocol: receive and send heights and send and receive units.
//
// The precise flow of this protocol follows:
/*		1. Receive a consistent snapshot of the other parties maximal units as a list of heights.
		2. Compute a similar info for our dag.
		3. Send this info.
		4. Compute and send units that are predecessors of our info and successors of the received.
		5. Receive units complying with the above restrictions.
		6. Add the received units to the dag.
*/
func (p *server) In() {
	conn, err := p.netserv.Listen()
	if err != nil {
		return
	}
	defer conn.Close()

	// receive a handshake
	pid, sid, err := handshake.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "gossip.in.greeting").Msg(err.Error())
		return
	}
	if pid >= p.nProc {
		p.log.Warn().Uint16(lg.PID, pid).Msg("Called by a stranger")
		return
	}

	if !p.peerManager.begin(pid) {
		return
	}
	defer p.peerManager.done(pid)

	log := p.log.With().Uint16(lg.PID, pid).Uint32(lg.ISID, sid).Logger()
	log.Info().Msg(lg.SyncStarted)

	// 1. receive dag info
	log.Debug().Msg(lg.GetInfo)
	theirDagInfo, err := encoding.ReadDagInfos(conn)
	if err != nil {
		log.Error().Str("where", "gossip.in.getDagInfo").Msg(err.Error())
		return
	}

	// 2. compute dag info
	dagInfo := p.orderer.GetInfo()

	// 3. send dag info
	log.Debug().Msg(lg.SendInfo)
	if err := encoding.WriteDagInfos(dagInfo, conn); err != nil {
		log.Error().Str("where", "gossip.in.sendDagInfo").Msg(err.Error())
		return
	}

	// 4. send units
	units := p.orderer.Delta(theirDagInfo)
	log.Debug().Int(lg.Sent, len(units)).Msg(lg.SendUnits)
	err = encoding.WriteChunk(units, conn)
	if err != nil {
		log.Error().Str("where", "gossip.in.sendUnits").Msg(err.Error())
		return
	}

	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "gossip.in.flush").Msg(err.Error())
		return
	}

	// 5. receive units
	log.Debug().Msg(lg.GetUnits)
	theirPreunitsReceived, err := encoding.ReadChunk(conn)
	nReceived := len(theirPreunitsReceived)
	if err != nil {
		log.Error().Str("where", "gossip.in.getPreunits").Msg(err.Error())
		return
	}

	// 6. add units
	errs := p.orderer.AddPreunits(pid, theirPreunitsReceived...)
	lg.AddingErrors(errs, len(theirPreunitsReceived), log)
	log.Info().Int(lg.Recv, nReceived).Int(lg.Sent, len(units)).Msg(lg.SyncCompleted)
}

// out handles the outgoing connection using info from the dag.
// This version uses 2-exchange simple protocol: send and receive heights and receive and send units.
//
// The precise flow of this protocol follows:
/*
    1. Get a consistent snapshot of our maximal units and convert it to a list of heights.
	2. Send this info.
	3. Receive a similar info created by the other party.
	4. Receive units, that are predecessors of the received info and successors of ours.
	5. Compute and send units complying with the above restrictions.
    6. Add the received units to the dag.
*/
func (p *server) Out() {
	remotePid, ok := p.peerManager.nextPeer()
	if !ok {
		return
	}
	defer p.peerManager.done(remotePid)

	conn, err := p.netserv.Dial(remotePid)
	if err != nil {
		return
	}
	defer conn.Close()

	// handshake
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	log := p.log.With().Uint16(lg.PID, remotePid).Uint32(lg.OSID, sid).Logger()
	log.Info().Msg(lg.SyncStarted)

	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		log.Error().Str("where", "gossip.out.greeting").Msg(err.Error())
		return
	}

	// 2. send dag info
	dagInfo := p.orderer.GetInfo()
	log.Debug().Msg(lg.SendInfo)
	if err := encoding.WriteDagInfos(dagInfo, conn); err != nil {
		log.Error().Str("where", "gossip.out.sendDagInfo").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "gossip.out.flush").Msg(err.Error())
		return
	}

	// 3. receive dag info
	log.Debug().Msg(lg.GetInfo)
	theirDagInfo, err := encoding.ReadDagInfos(conn)
	if err != nil {
		// errors here happen when the remote side rejects our gossip attempt, hence they are not "true" errors
		log.Debug().Str("where", "gossip.out.getDagInfo").Msg(err.Error())
		return
	}

	// 4. receive units
	log.Debug().Msg(lg.GetUnits)
	theirPreunitsReceived, err := encoding.ReadChunk(conn)
	nReceived := len(theirPreunitsReceived)
	if err != nil {
		log.Error().Str("where", "gossip.out.getPreunits").Msg(err.Error())
		return
	}

	// 5. send units
	units := p.orderer.Delta(theirDagInfo)
	log.Debug().Int(lg.Sent, len(units)).Msg(lg.SendUnits)
	err = encoding.WriteChunk(units, conn)
	if err != nil {
		log.Error().Str("where", "gossip.out.sendUnits").Msg(err.Error())
		return
	}
	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "gossip.out.flush2").Msg(err.Error())
		return
	}

	// 6. add units to dag
	errs := p.orderer.AddPreunits(remotePid, theirPreunitsReceived...)
	lg.AddingErrors(errs, len(theirPreunitsReceived), log)
	log.Info().Int(lg.Recv, nReceived).Int(lg.Sent, len(units)).Msg(lg.SyncCompleted)
}
