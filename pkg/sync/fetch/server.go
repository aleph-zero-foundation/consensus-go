// Package fetch implements a mechanism of fetching specific units with known hashes.
//
// This protocol cannot be used for general syncing, because usually we don't know the hashes of units we would like to receive in advance.
// It is only useful as a fallback mechanism.
package fetch

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

type server struct {
	pid      uint16
	dag      gomel.Dag
	adder    gomel.Adder
	netserv  network.Server
	fallback sync.Fallback
	requests chan request
	syncIds  []uint32
	outPool  sync.WorkerPool
	inPool   sync.WorkerPool
	timeout  time.Duration
	log      zerolog.Logger
}

// NewServer runs a pool of nOut workers for outgoing part and nIn for incoming part of the given protocol
func NewServer(pid uint16, dag gomel.Dag, adder gomel.Adder, netserv network.Server, timeout time.Duration, log zerolog.Logger, nOut, nIn int) (sync.Server, sync.Fallback) {
	nProc := int(dag.NProc())
	requests := make(chan request, nProc)
	s := &server{
		pid:      pid,
		dag:      dag,
		adder:    adder,
		netserv:  netserv,
		requests: requests,
		syncIds:  make([]uint32, nProc),
		timeout:  timeout,
		log:      log,
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

// Resolve builds a fetch request containing all the unknown parents of a problematic preunit.
func (s *server) Resolve(preunit gomel.Preunit) {
	topHeights := gomel.DealingHeights(s.dag.NProc())
	s.dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			if u.Height() > topHeights[u.Creator()] {
				topHeights[u.Creator()] = u.Height()
			}
		}
		return true
	})

	s.requests <- request{
		pid:     preunit.Creator(),
		heights: topHeights,
	}
}
