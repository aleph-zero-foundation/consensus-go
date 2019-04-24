package network

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

const (
	N_INSYNC  = 10
	N_OUTSYNC = 10
)

// Syncer retrieves ready-to-use connections and dispatches workers that use
// the connections for running in/out synchronizations according to a sync-protocol
type Syncer struct {
	poset       gomel.Poset
	inConnChan  chan Connection
	outConnChan chan Connection
	inProto     sync.In
	outProto    sync.In
	inSem       chan struct{}
	outSem      chan struct{}
	exitChan    chan struct{}
}

// NewSyncer needs a local poset and sources of in/out connections.
func NewSyncer(poset gomel.Poset, inConnChan, outConnChan chan Connection, inSyncProto sync.In, outSyncProto sync.Out) *Syncer {
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
	go syncDispatcher(s.inConnChan, s.inSem, s.inSyncProto.Run)
	go syncDispatcher(s.outConnChan, s.outSem, s.outSyncProto.Run)
}

func (s *Syncer) Stop() {
	close(s.exitChan)
}

func (s *Syncer) syncDispatcher(connChan chan network.Connection, sem chan struct{}, syncProto func(poset gomel.Poset, conn network.Connection)) {
	for {
		select {
		case <-s.exitChan:
			// clean things up
			return
		case conn <- ConnChan:
			sem <- struct{}{}
			go syncProto(s.poset, conn)
		}
	}
}
