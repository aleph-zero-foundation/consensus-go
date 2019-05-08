package sync

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

const (
	nInitSync = 10 // todo: pull from config
	nRecvSync = 10 // todo: pull from config
)

// Server retrieves ready-to-use connections and dispatches workers that use
// the connections for running in/out synchronizations according to a sync-protocol
type Server struct {
	poset        gomel.Poset
	inConnChan   chan network.Connection
	outConnChan  chan network.Connection
	inSyncProto  Protocol
	outSyncProto Protocol
	exitChan     chan struct{}
}

// NewServer needs a local poset and sources of in/out connections.
func NewServer(poset gomel.Poset, inConnChan, outConnChan chan network.Connection, inSyncProto Protocol, outSyncProto Protocol) *Server {
	return &Server{
		poset:        poset,
		inConnChan:   inConnChan,
		outConnChan:  outConnChan,
		inSyncProto:  inSyncProto,
		outSyncProto: outSyncProto,
		exitChan:     make(chan struct{}),
	}
}

// Start starts server
func (s *Server) Start() {
	for i := 0; i < nInitSync; i++ {
		go s.syncDispatcher(s.inConnChan, s.inSyncProto.Run)
	}
	for i := 0; i < nRecvSync; i++ {
		go s.syncDispatcher(s.outConnChan, s.outSyncProto.Run)
	}
}

// Stop stops server
func (s *Server) Stop() {
	close(s.exitChan)
}

func (s *Server) syncDispatcher(connChan chan network.Connection, syncProto func(poset gomel.Poset, conn network.Connection)) {
	for {
		select {
		case <-s.exitChan:
			// clean things up
			return
		case conn := <-connChan:
			syncProto(s.poset, conn)
		}
	}
}
