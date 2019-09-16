// Package gossip implements a protocol for synchronising dags through gossiping.
//
// This protocol should always succeed with adding units received from honest peers, so it needs no fallback.
package gossip

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

type server struct {
	pid          uint16
	dag          gomel.Dag
	randomSource gomel.RandomSource
	netserv      network.Server
	fallback     sync.QueryServer
	requests     chan uint16
	peerSource   PeerSource
	inUse        []*mutex
	syncIds      []uint32
	outPool      sync.WorkerPool
	inPool       sync.WorkerPool
	timeout      time.Duration
	log          zerolog.Logger
}

// NewServer runs a pool of nOut workers for the outgoing part and nIn for the incoming part of the gossip protocol.
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, netserv network.Server, timeout time.Duration, log zerolog.Logger, nOut, nIn int) sync.QueryServer {
	nProc := int(dag.NProc())
	inUse := make([]*mutex, nProc)
	for i := range inUse {
		inUse[i] = newMutex()
	}
	requests := make(chan uint16, nProc)
	s := &server{
		pid:          pid,
		dag:          dag,
		randomSource: randomSource,
		netserv:      netserv,
		requests:     requests,
		peerSource:   NewMixedPeerSource(dag.NProc(), pid, requests),
		inUse:        inUse,
		syncIds:      make([]uint32, nProc),
		timeout:      timeout,
		log:          log,
	}
	s.outPool = sync.NewPool(nOut, s.Out)
	s.inPool = sync.NewPool(nIn, s.In)
	return s
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	close(s.requests)
	s.outPool.Stop()
}

func (s *server) SetFallback(qs sync.QueryServer) {
	s.fallback = qs
}

// FindOut requests next gossip to happen with the creator of a problematic preunit.
func (s *server) FindOut(preunit gomel.Preunit) {
	select {
	case s.requests <- preunit.Creator():
	default:
	}
}
