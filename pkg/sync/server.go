package sync

import (
	"github.com/rs/zerolog"
)

// Server receives ready-to-use incoming connections and establishes outgoing ones,
// to later handle them using the provided protocols.
type Server struct {
	proto   Protocol
	outPool *pool
	inPool  *pool
	log     zerolog.Logger
}

// NewServer constructs a server for the given dag, channels of incoming and outgoing connections, protocols for connection handling,
// and maximal numbers of syncs to initialize and receive.
func NewServer(proto Protocol, nOut, nIn uint, log zerolog.Logger) *Server {
	return &Server{
		proto:   proto,
		outPool: newPool(nOut, proto.Out),
		inPool:  newPool(nIn, proto.In),
		log:     log,
	}
}

// Start starts server
func (s *Server) Start() {
	s.outPool.start()
	s.inPool.start()
}

// StopIn stops handling incoming synchronizations
func (s *Server) StopIn() {
	s.inPool.stop()
}

// StopOut stops handling outgoing synchronizations
func (s *Server) StopOut() {
	s.outPool.stop()
}
