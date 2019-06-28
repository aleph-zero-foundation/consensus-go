package sync

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// Server receives ready-to-use incoming connections and establishes outgoing ones,
// to later handle them using the provided protocols.
type Server struct {
	inConnChan  <-chan network.Connection
	proto       Protocol
	nOutSync    uint
	nInSync     uint
	exitChanIn  chan struct{}
	exitChanOut chan struct{}
	wgIn        sync.WaitGroup
	wgOut       sync.WaitGroup
	log         zerolog.Logger
}

// NewServer constructs a server for the given poset, channels of incoming and outgoing connections, protocols for connection handling,
// and maximal numbers of syncs to initialize and receive.
func NewServer(inConnChan <-chan network.Connection, proto Protocol, nOutSync, nInSync uint, log zerolog.Logger) *Server {
	return &Server{
		inConnChan:  inConnChan,
		proto:       proto,
		nOutSync:    nOutSync,
		nInSync:     nInSync,
		exitChanIn:  make(chan struct{}),
		exitChanOut: make(chan struct{}),
		log:         log,
	}
}

// Start starts server
func (s *Server) Start() {
	s.wgIn.Add(int(s.nInSync))
	for i := uint(0); i < s.nInSync; i++ {
		go s.inDispatcher()
	}
	s.wgOut.Add(int(s.nOutSync))
	for i := uint(0); i < s.nOutSync; i++ {
		go s.outDispatcher()
	}
}

// StopIn stops handling incoming synchronizations
func (s *Server) StopIn() {
	close(s.exitChanIn)
	s.wgIn.Wait()
}

// StopOut stops handling outgoing synchronizations
func (s *Server) StopOut() {
	close(s.exitChanOut)
	s.wgOut.Wait()
}

func (s *Server) inDispatcher() {
	defer s.wgIn.Done()
	for {
		select {
		case <-s.exitChanIn:
			return
		case conn, ok := <-s.inConnChan:
			if !ok {
				return
			}
			s.proto.In(conn)
		}
	}
}

func (s *Server) outDispatcher() {
	defer s.wgOut.Done()
	for {
		select {
		case <-s.exitChanOut:
			return
		default:
			s.proto.Out()
		}
	}
}
