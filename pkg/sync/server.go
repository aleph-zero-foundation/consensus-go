package sync

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// Server retrieves ready-to-use connections and dispatches workers that use
// the connections for running in/out synchronizations according to a sync-protocol
type Server struct {
	poset        gomel.Poset
	inConnChan   <-chan network.Connection
	outConnChan  <-chan network.Connection
	inSyncProto  Protocol
	outSyncProto Protocol
	nInitSync    uint
	nRecvSync    uint
	exitChan     chan struct{}
	wg           sync.WaitGroup
}

// NewServer constructs a server for the given poset, channels of incoming and outgoing connections, protocols for connection handling,
// and maximal numbers of syncs to initialize and receive.
func NewServer(poset gomel.Poset, inConnChan, outConnChan <-chan network.Connection, inSyncProto, outSyncProto Protocol, nInitSync, nRecvSync uint) *Server {
	return &Server{
		poset:        poset,
		inConnChan:   inConnChan,
		outConnChan:  outConnChan,
		inSyncProto:  inSyncProto,
		outSyncProto: outSyncProto,
		nInitSync:    nInitSync,
		nRecvSync:    nRecvSync,
		exitChan:     make(chan struct{}),
	}
}

// Start starts server
func (s *Server) Start() {
	for i := uint(0); i < s.nInitSync; i++ {
		s.wg.Add(1)
		go s.syncDispatcher(s.inConnChan, s.inSyncProto.Run)
	}
	for i := uint(0); i < s.nRecvSync; i++ {
		s.wg.Add(1)
		go s.syncDispatcher(s.outConnChan, s.outSyncProto.Run)
	}
}

// Stop stops server
func (s *Server) Stop() {
	close(s.exitChan)
	s.wg.Wait()
}

func (s *Server) syncDispatcher(connChan <-chan network.Connection, syncProto func(poset gomel.Poset, conn network.Connection)) {
	defer s.wg.Done()
	for {
		select {
		case <-s.exitChan:
			return
		case conn, ok := <-connChan:
			if !ok {
				<-s.exitChan
				return
			}
			syncProto(s.poset, conn)
		}
	}
}
