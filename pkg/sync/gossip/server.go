package gossip

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"time"
)

// NewServer runs a pool of nOut workers for outgoing part and nIn for incoming part of the given protocol
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
