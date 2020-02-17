// Package fetch implements a mechanism of fetching specific units with known hashes.
//
// This protocol cannot be used for general syncing, because usually we don't know the hashes of units we would like to receive in advance.
// It is only useful as a fallback mechanism.
package fetch

import (
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/core-go/pkg/network"
)

type server struct {
	pid      uint16
	orderer  gomel.Orderer
	netserv  network.Server
	requests chan Request
	syncIds  []uint32
	outPool  sync.WorkerPool
	inPool   sync.WorkerPool
	timeout  time.Duration
	quit     int64
	log      zerolog.Logger
}

// NewServer runs a pool of nOut workers for outgoing part and nIn for incoming part of the given protocol
func NewServer(conf config.Config, orderer gomel.Orderer, netserv network.Server, log zerolog.Logger, nOut, nIn int, timeout time.Duration) (sync.Server, sync.Fetch) {
	nProc := int(conf.NProc)
	requests := make(chan Request, nProc)
	s := &server{
		pid:      conf.Pid,
		orderer:  orderer,
		netserv:  netserv,
		requests: requests,
		syncIds:  make([]uint32, nProc),
		timeout:  timeout,
		log:      log,
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

func (s *server) trigger(pid uint16, ids []uint64) {
	if atomic.LoadInt64(&s.quit) == 0 {
		s.requests <- Request{pid, ids}
	}
}
