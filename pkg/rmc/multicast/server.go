package multicast

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer returns a server
func NewServer(pid uint16, nProc int, state *rmc.RMC, requests chan Request, accepted chan []byte, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) *Server {

	proto := newProtocol(pid, nProc, requests, state, accepted, dialer, listener, timeout, log)
	return &Server{
		requests: requests,
		outPool:  sync.NewPool(uint(5), proto.Out),
		inPool:   sync.NewPool(uint(5), proto.In),
	}
}

type Server struct {
	requests chan Request
	outPool  *sync.Pool
	inPool   *sync.Pool
}

func (s *Server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *Server) StopIn() {
	s.inPool.Stop()
}

func (s *Server) StopOut() {
	close(s.requests)
	s.outPool.Stop()
}
