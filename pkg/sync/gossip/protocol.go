package gossip

import (
	"gitlab.com/alephledger/consensus-go/pkg/logging"
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
		p.log.Error().Str("where", "gossip.in.greeting").Msg(err.Error())
		return
	}
	if int(pid) >= len(p.inUse) {
		p.log.Warn().Uint16(logging.PID, pid).Msg("Called by a stranger")
		return
	}
	m := p.inUse[pid]
	if !m.tryAcquire() {
		return
	}
	defer m.release()
	conn.SetLogger(p.log.With().Uint16(logging.PID, pid).Uint32(logging.ISID, sid).Logger())
	p.inExchange(conn)
}

func (p *server) out() {
	remotePid := p.peerSource.NextPeer()
	m := p.inUse[remotePid]
	if !m.tryAcquire() {
		return
	}
	defer m.release()
	conn, err := p.netserv.Dial(remotePid, p.timeout)
	if err != nil {
		p.log.Error().Str("where", "gossip.out.dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		p.log.Error().Str("where", "gossip.out.greeting").Msg(err.Error())
		return
	}
	conn.SetLogger(p.log.With().Uint16(logging.PID, remotePid).Uint32(logging.OSID, sid).Logger())
	p.outExchange(conn)
}
