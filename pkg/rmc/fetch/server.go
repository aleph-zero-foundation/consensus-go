package fetch

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer returns a server that runs fetch for rmc protocol
func NewServer(pid uint16, dag gomel.Dag, rs gomel.RandomSource, state *rmc.RMC, requests chan gomel.Preunit, netserv network.Server, timeout time.Duration, log zerolog.Logger) *Server {
	proto := NewProtocol(pid, dag, rs, requests, netserv, timeout, log)
	return &Server{
		requests: requests,
		outPool:  sync.NewPool(uint(5), proto.Out),
		inPool:   sync.NewPool(uint(5), proto.In),
	}

}

// Server is a server for rmc fetch
type Server struct {
	requests chan gomel.Preunit
	outPool  *sync.Pool
	inPool   *sync.Pool
}

// Start starts the server
func (s *Server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

// StopIn stops incoming connections
func (s *Server) StopIn() {
	s.inPool.Stop()
}

// StopOut stops outgoing connections
func (s *Server) StopOut() {
	s.outPool.Stop()
}
