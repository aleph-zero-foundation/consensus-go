package fetch

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer returns a server
func NewServer(pid uint16, dag gomel.Dag, rs gomel.RandomSource, state *rmc.State, requests chan gomel.Preunit, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) *Server {
	proto := newProtocol(pid, dag, rs, requests, state, dialer, listener, timeout, log)
	return &Server{
		requests: requests,
		outPool:  sync.NewPool(uint(5), proto.Out),
		inPool:   sync.NewPool(uint(5), proto.In),
	}

}

type Server struct {
	requests chan gomel.Preunit
	outPool  *sync.Pool
	inPool   *sync.Pool
}

func (s *Server) Start() {
	s.outPool.Start()
	s.inPool.Start()
	fmt.Println("FECZ STARTED")
}

func (s *Server) StopIn() {
	s.inPool.Stop()
}

func (s *Server) StopOut() {
	close(s.requests)
	s.outPool.Stop()
}
