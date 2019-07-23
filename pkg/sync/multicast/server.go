package multicast

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer returns a server that runs multicast protocol and callback for create service
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, dialer network.Dialer, listener network.Listener, timeout time.Duration, fallback sync.Fallback, log zerolog.Logger) (sync.Server, func(gomel.Unit)) {
	proto := newProtocol(pid, dag, randomSource, dialer, listener, timeout, fallback, log)
	return &server{
			proto:   proto,
			outPool: sync.NewPool(uint(mcOutWPSize*dag.NProc()), proto.Out),
			inPool:  sync.NewPool(uint(mcInWPSize*dag.NProc()), proto.In),
		}, func(unit gomel.Unit) {
			proto.request(unit)
		}
}

type server struct {
	proto   *protocol
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
	close(s.proto.requests)
	s.outPool.Stop()
}
