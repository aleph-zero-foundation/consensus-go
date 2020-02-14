// Package gossip implements a protocol for synchronizing dags through gossiping.
//
// This protocol should always succeed with adding units received from honest peers, so it needs no fallback.
package gossip

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/core-go/pkg/network"
)

type server struct {
	pid         uint16
	orderer     gomel.Orderer
	netserv     network.Server
	peerManager *peerManager
	syncIds     []uint32
	outPool     sync.WorkerPool
	inPool      sync.WorkerPool
	timeout     time.Duration
	log         zerolog.Logger
}

// NewServer runs a pool of nOut workers for the outgoing part and nIn for the incoming part of the gossip protocol.
func NewServer(conf config.Config, orderer gomel.Orderer, netserv network.Server, timeout time.Duration, log zerolog.Logger, nOut, nIn, nIdle int) (sync.Server, sync.RequestGossip) {
	pid := conf.Pid
	nProc := int(conf.NProc)
	s := &server{
		pid:         pid,
		orderer:     orderer,
		netserv:     netserv,
		peerManager: newPeerManager(conf.NProc, pid, nIdle),
		syncIds:     make([]uint32, nProc),
		timeout:     timeout,
		log:         log,
	}
	s.outPool = sync.NewPool(nOut, s.Out)
	s.inPool = sync.NewPool(nIn, s.In)
	return s, s.peerManager.request
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	s.peerManager.stop()
	s.outPool.Stop()
}
