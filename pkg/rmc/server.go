package rmc

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer returns a server
func NewServer(pid uint16, pubs []*bn256.VerificationKey, priv *bn256.SecretKey, requests chan Request, accepted chan []byte, dialer network.Dialer, listener network.Listener, timeout time.Duration, log zerolog.Logger) *Server {
	state := New(pubs, priv)
	proto := newProtocol(pid, len(pubs), requests, state, accepted, dialer, listener, timeout, log)
	return &Server{
		requests: requests,
		outPool:  sync.NewPool(uint(50), proto.Out),
		inPool:   sync.NewPool(uint(50), proto.In),
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
