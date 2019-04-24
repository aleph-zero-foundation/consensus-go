package sync

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

const (
	N_INSYNC  = 10
	N_OUTSYNC = 10
)

// Syncer retrieves ready-to-use connections and dispatches workers that use
// the connections for running in/out synchronizations according to a sync-protocol
type Syncer struct {
	poset        gomel.Poset
	inConnChan   chan network.Connection
	outConnChan  chan network.Connection
	inSyncProto  In
	outSyncProto In
	inSem        chan struct{}
	outSem       chan struct{}
	exitChan     chan struct{}
}

// NewSyncer needs a local poset and sources of in/out connections.
func NewSyncer(poset gomel.Poset, inConnChan, outConnChan chan network.Connection, inSyncProto In, outSyncProto Out) *Syncer {
	cs := &Syncer{
		poset:        poset,
		inConnChan:   inConnChan,
		outConnChan:  outConnChan,
		inSyncProto:  inSyncProto,
		outSyncProto: outSyncProto,
		inSem:        make(chan struct{}, N_INSYNC),
		outSem:       make(chan struct{}, N_OUTSYNC),
		exitChan:     make(chan struct{}),
	}

	cs.inSyncProto.OnDone(func() {
		<-cs.inSem
	})
	cs.outSyncProto.OnDone(func() {
		<-cs.outSem
	})

	return cs
}

// Start starts syncer
func (s *Syncer) Start() {
	go s.syncDispatcher(s.inConnChan, s.inSem, s.inSyncProto.Run)
	go s.syncDispatcher(s.outConnChan, s.outSem, s.outSyncProto.Run)
}

// Stop stops syncer
func (s *Syncer) Stop() {
	close(s.exitChan)
}

func (s *Syncer) syncDispatcher(connChan chan network.Connection, sem chan struct{}, syncProto func(poset gomel.Poset, conn network.Connection)) {
	for {
		select {
		case <-s.exitChan:
			// clean things up
			return
		case conn := <-connChan:
			sem <- struct{}{}
			go syncProto(s.poset, conn)
		}
	}
}
