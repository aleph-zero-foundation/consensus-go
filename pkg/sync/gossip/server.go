// Package gossip implements a protocol for synchronizing dags through gossiping.
//
// This protocol should always succeed with adding units received from honest peers, so it needs no fallback.
package gossip

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/core-go/pkg/network"
)

type server struct {
	nProc    uint16
	pid      uint16
	orderer  gomel.Orderer
	netserv  network.Server
	requests chan uint16
	syncIds  []uint32
	tokens   []chan struct{}
	outPool  sync.WorkerPool
	inPool   sync.WorkerPool
	stopOut  chan struct{}
	log      zerolog.Logger
}

// NewServer runs a pool of nOut workers for the outgoing part and nIn for the incoming part of the gossip protocol.
func NewServer(conf config.Config, orderer gomel.Orderer, netserv network.Server, log zerolog.Logger) (sync.Server, sync.Gossip) {
	s := &server{
		nProc:    conf.NProc,
		pid:      conf.Pid,
		orderer:  orderer,
		netserv:  netserv,
		requests: make(chan uint16, conf.NProc),
		syncIds:  make([]uint32, conf.NProc),
		tokens:   make([]chan struct{}, conf.NProc),
		stopOut:  make(chan struct{}),
		log:      log,
	}
	for i := range s.tokens {
		s.tokens[i] = make(chan struct{}, 1)
		s.tokens[i] <- struct{}{}
	}
	s.inPool = sync.NewPool(conf.GossipWorkers[0], s.In)
	s.outPool = sync.NewPool(conf.GossipWorkers[1], s.Out)
	return s, s.request
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	close(s.stopOut)
	s.outPool.Stop()
}

func (s *server) request(pid uint16) {
	select {
	case s.requests <- pid:
	default:
		s.log.Warn().Msg(lg.RequestOverload)
	}
}
