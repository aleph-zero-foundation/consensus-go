// Package gossip implements a protocol for synchronizing dags through gossiping.
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
	pid        uint16
	dag        gomel.Dag
	adder      gomel.Adder
	netserv    network.Server
	fallback   sync.Fallback
	requests   chan uint16
	peerSource PeerSource
	inUse      []*mutex
	syncIds    []uint32
	outPool    sync.WorkerPool
	inPool     sync.WorkerPool
	timeout    time.Duration
	log        zerolog.Logger
}

// NewServer runs a pool of nOut workers for the outgoing part and nIn for the incoming part of the gossip protocol.
func NewServer(pid uint16, dag gomel.Dag, adder gomel.Adder, netserv network.Server, timeout time.Duration, log zerolog.Logger, nOut, nIn int) (sync.Server, sync.Fallback) {
	nProc := int(dag.NProc())
	inUse := make([]*mutex, nProc)
	for i := range inUse {
		inUse[i] = newMutex()
	}
	requests := make(chan uint16, 5*nOut)
	s := &server{
		pid:        pid,
		dag:        dag,
		adder:      adder,
		netserv:    netserv,
		requests:   requests,
		peerSource: NewMixedPeerSource(dag.NProc(), pid, requests),
		inUse:      inUse,
		syncIds:    make([]uint32, nProc),
		timeout:    timeout,
		log:        log,
	}
	s.outPool = sync.NewPool(nOut, s.out)
	s.inPool = sync.NewPool(nIn, s.in)
	return s, s
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

func (s *server) SetFallback(qs sync.Fallback) {
	s.fallback = qs
}

// Resolve requests next gossip to happen with the creator of a problematic preunit.
func (s *server) Resolve(preunit gomel.Preunit) {
	select {
	case s.requests <- preunit.Creator():
	default:
	}
}
