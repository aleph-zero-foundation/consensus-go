package multicast

import (
	"time"

	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer returns a server that runs rmc protocol
func NewServer(pid uint16, dag gomel.Dag, state *rmc.RMC, requests chan *Request, canMulticast *sync.Mutex, accepted chan []byte, netserv network.Server, timeout time.Duration, log zerolog.Logger) *Server {

	proto := newProtocol(pid, dag, requests, state, canMulticast, accepted, netserv, timeout, log)
	return &Server{
		requests: requests,
		outPool:  gsync.NewPool(uint(dag.NProc()), proto.Out),
		inPool:   gsync.NewPool(uint(dag.NProc()), proto.In),
	}
}

// Server is a multicast server
type Server struct {
	requests chan *Request
	outPool  *gsync.Pool
	inPool   *gsync.Pool
}

// Start starts worker pools
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
	close(s.requests)
	s.outPool.Stop()
}
