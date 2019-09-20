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
	fallback sync.QueryServer
	requests chan request
	syncIds  []uint32
	outPool  sync.WorkerPool
	inPool   sync.WorkerPool
	timeout  time.Duration
	log      zerolog.Logger
}

// NewServer runs a pool of nOut workers for outgoing part and nIn for incoming part of the given protocol
func NewServer(pid uint16, dag gomel.Dag, adder gomel.Adder, netserv network.Server, timeout time.Duration, log zerolog.Logger, nOut, nIn int) sync.QueryServer {
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

// FindOut builds a fetch request containing all the unknown parents of a problematic preunit.
func (s *server) FindOut(preunit gomel.Preunit) {
	hashes := preunit.Parents()
	parents := s.dag.Get(hashes)
	toRequest := []*gomel.Hash{}
	for i, h := range hashes {
		if parents[i] == nil {
			toRequest = append(toRequest, h)
		}
	}
	if len(toRequest) > 0 {
		select {
		case s.requests <- request{
			pid:    preunit.Creator(),
			hashes: toRequest,
		}:
		default:
		}
	}
}
