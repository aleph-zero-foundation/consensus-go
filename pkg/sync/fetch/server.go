package fetch

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"time"
)

// NewServer runs a pool of nOut workers for outgoing part and nIn for incoming part of the given protocol
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, reqs chan Request, dialer network.Dialer, listener network.Listener, callback gomel.Callback, timeout time.Duration, fallback sync.Fallback, log zerolog.Logger, nOut, nIn uint) sync.Server {
	proto := NewProtocol(pid, dag, randomSource, reqs, dialer, listener, callback, timeout, fallback, log)
	return &server{
		reqs:    reqs,
		outPool: sync.NewPool(nOut, proto.Out),
		inPool:  sync.NewPool(nIn, proto.In),
	}
}

type server struct {
	reqs    chan Request
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
	close(s.reqs)
	s.outPool.Stop()
}
