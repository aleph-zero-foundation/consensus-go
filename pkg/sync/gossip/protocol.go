package gossip

import (
	"time"

	"github.com/rs/zerolog"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

type protocol struct {
	pid           uint16
	poset         gomel.Poset
	randomSource  gomel.RandomSource
	peerSource    *peerSource
	dialer        network.Dialer
	listener      network.Listener
	inUse         []*mutex
	syncIds       []uint32
	timeout       time.Duration
	attemptTiming chan<- int
	log           zerolog.Logger
}

// NewProtocol returns a new gossiping protocol.
func NewProtocol(pid uint16, poset gomel.Poset, randomSource gomel.RandomSource, dialer network.Dialer, listener network.Listener, timeout time.Duration, attemptTiming chan<- int, log zerolog.Logger) sync.Protocol {
	nProc := uint16(dialer.Length())
	peerSource := newPeerSource(nProc, pid)
	inUse := make([]*mutex, nProc)
	for i := range inUse {
		inUse[i] = newMutex()
	}
	return &protocol{
		pid:           pid,
		poset:         poset,
		randomSource:  randomSource,
		peerSource:    peerSource,
		dialer:        dialer,
		listener:      listener,
		inUse:         inUse,
		syncIds:       make([]uint32, nProc),
		timeout:       timeout,
		attemptTiming: attemptTiming,
		log:           log,
	}
}

func (p *protocol) In() {
	conn, err := p.listener.Listen()
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	pid, sid, err := handshake.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "gossipProtocol.in.greeting").Msg(err.Error())
		return
	}
	if pid >= uint16(len(p.inUse)) {
		p.log.Warn().Uint16(logging.PID, pid).Msg("Called by a stranger")
		return
	}
	m := p.inUse[pid]
	if !m.tryAcquire() {
		return
	}
	defer m.release()
	conn.SetLogger(p.log.With().Uint16(logging.PID, pid).Uint32(logging.ISID, sid).Logger())
	inExchange(p.poset, p.randomSource, p.attemptTiming, conn)
}

func (p *protocol) Out() {
	remotePid := p.peerSource.nextPeer()
	m := p.inUse[remotePid]
	if !m.tryAcquire() {
		return
	}
	defer m.release()
	conn, err := p.dialer.Dial(remotePid)
	if err != nil {
		p.log.Error().Str("where", "gossipProtocol.out.dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		p.log.Error().Str("where", "gossipProtocol.out.greeting").Msg(err.Error())
		return
	}
	conn.SetLogger(p.log.With().Int(logging.PID, int(remotePid)).Uint32(logging.OSID, sid).Logger())
	outExchange(p.poset, p.randomSource, p.attemptTiming, conn)
}
