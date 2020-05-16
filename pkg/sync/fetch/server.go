// Package fetch implements a mechanism of fetching specific units with known hashes.
//
// This protocol cannot be used for general syncing, because usually we don't know the hashes of units we would like to receive in advance.
// It is only useful as a fallback mechanism.
package fetch

import (
	gsync "sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/core-go/pkg/network"
)

type server struct {
	pid      uint16
	orderer  gomel.Orderer
	netserv  network.Server
	requests chan request
	syncIds  []uint32
	outPool  sync.WorkerPool
	inPool   sync.WorkerPool
	mx       gsync.RWMutex
	closed   bool
	log      zerolog.Logger
}

// NewServer runs a pool of nOut workers for outgoing part and nIn for incoming part of the given protocol
func NewServer(conf config.Config, orderer gomel.Orderer, netserv network.Server, log zerolog.Logger) (sync.Server, sync.Fetch) {
	s := &server{
		pid:      conf.Pid,
		orderer:  orderer,
		netserv:  netserv,
		requests: make(chan request, conf.NProc),
		syncIds:  make([]uint32, conf.NProc),
		log:      log,
	}
	s.inPool = sync.NewPool(conf.FetchWorkers[0], s.In)
	s.outPool = sync.NewPool(conf.FetchWorkers[1], s.Out)
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
	s.mx.Lock()
	defer s.mx.Unlock()
	s.closed = true
	close(s.requests)
	s.outPool.Stop()
}

func (s *server) trigger(pid uint16, ids []uint64) {
	s.mx.RLock()
	defer s.mx.RUnlock()
	if !s.closed {
		select {
		case s.requests <- request{pid, ids}:
		default:
			s.log.Warn().Msg(lg.RequestOverload)
		}
	}
}
