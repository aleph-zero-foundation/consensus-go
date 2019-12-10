// Package gossip implements a protocol for synchronizing dags through gossiping.
//
// This protocol should always succeed with adding units received from honest peers, so it needs no fallback.
package gossip

import (
	"sync/atomic"
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
	requests   chan uint16
	peerSource PeerSource
	syncIds    []uint32
	outPool    sync.WorkerPool
	inPool     sync.WorkerPool
	timeout    time.Duration
	quit       int64
	log        zerolog.Logger
}

// NewServer runs a pool of nOut workers for the outgoing part and nIn for the incoming part of the gossip protocol.
func NewServer(pid uint16, dag gomel.Dag, adder gomel.Adder, netserv network.Server, timeout time.Duration, log zerolog.Logger, nOut, nIn int) (sync.Server, gomel.RequestGossip) {
	nProc := int(dag.NProc())
	requests := make(chan uint16, 5*nOut)
	s := &server{
		pid:      pid,
		dag:      dag,
		adder:    adder,
		netserv:  netserv,
		requests: requests,
		//peerSource: NewMixedPeerSource(dag.NProc(), pid, requests),
		syncIds: make([]uint32, nProc),
		timeout: timeout,
		log:     log,
	}
	s.outPool = sync.NewPool(nOut, s.Out)
	s.inPool = sync.NewPool(nIn, s.In)
	return s, s.trigger
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	atomic.StoreInt64(&s.quit, 1)
	close(s.requests)
	s.outPool.Stop()
}

func (s *server) trigger(pid uint16) {
	if atomic.LoadInt64(&s.quit) == 0 {
		s.requests <- pid
	}
}
