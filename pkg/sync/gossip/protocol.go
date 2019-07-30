package gossip

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/handshake"
)

type protocol struct {
	pid          uint16
	dag          gomel.Dag
	randomSource gomel.RandomSource
	dialer       network.Dialer
	listener     network.Listener
	peerSource   PeerSource
	callback     gomel.Callback
	timeout      time.Duration
	log          zerolog.Logger
	inUse        []*mutex
	syncIds      []uint32
}

// NewProtocol returns a new gossiping protocol.
func NewProtocol(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, dialer network.Dialer, listener network.Listener, peerSource PeerSource, callback gomel.Callback, timeout time.Duration, log zerolog.Logger) sync.Protocol {
	nProc := uint16(dag.NProc())
	inUse := make([]*mutex, nProc)
	for i := range inUse {
		inUse[i] = newMutex()
	}
	return &protocol{
		pid:          pid,
		dag:          dag,
		randomSource: randomSource,
		dialer:       dialer,
		listener:     listener,
		peerSource:   peerSource,
		callback:     callback,
		timeout:      timeout,
		log:          log,
		inUse:        inUse,
		syncIds:      make([]uint32, nProc),
	}
}

func (p *protocol) In() {
	conn, err := p.listener.Listen(p.timeout)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	pid, sid, err := handshake.AcceptGreeting(conn)
	if err != nil {
		p.log.Error().Str("where", "gossip.In.greeting").Msg(err.Error())
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
	p.inExchange(conn)
}

func (p *protocol) Out() {
	remotePid := p.peerSource.NextPeer()
	m := p.inUse[remotePid]
	if !m.tryAcquire() {
		return
	}
	defer m.release()
	conn, err := p.dialer.Dial(remotePid)
	if err != nil {
		p.log.Error().Str("where", "gossip.Out.dial").Msg(err.Error())
		return
	}
	defer conn.Close()
	conn.TimeoutAfter(p.timeout)
	sid := p.syncIds[remotePid]
	p.syncIds[remotePid]++
	err = handshake.Greet(conn, p.pid, sid)
	if err != nil {
		p.log.Error().Str("where", "gossip.Out.greeting").Msg(err.Error())
		return
	}
	conn.SetLogger(p.log.With().Int(logging.PID, int(remotePid)).Uint32(logging.OSID, sid).Logger())
	p.outExchange(conn)
}
