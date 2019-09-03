package gossip

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer runs a pool of nOut workers for the outgoing part and nIn for the incoming part of the gossip protocol.
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, netserv network.Server, peerSource PeerSource, callback gomel.Callback, timeout time.Duration, log zerolog.Logger, nOut, nIn uint) sync.Server {
	proto := NewProtocol(pid, dag, randomSource, netserv, peerSource, callback, timeout, log)
	return &server{
		outPool: sync.NewPool(nOut, proto.Out),
		inPool:  sync.NewPool(nIn, proto.In),
	}
}

type server struct {
	outPool *sync.Pool
	inPool  *sync.Pool
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	s.outPool.Stop()
}
