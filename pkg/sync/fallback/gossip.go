package fallback

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

type gossipFallback struct {
	requests chan<- uint16
}

func (f *gossipFallback) Run(pu gomel.Preunit) {
	f.requests <- pu.Creator()
}

func (f *gossipFallback) Stop() {
	close(f.requests)
}

// NewGossip pushes a request for a sync with the creator of the problematic unit to the provided channel.
func NewGossip(requests chan<- uint16) sync.Fallback {
	return &gossipFallback{
		requests: requests,
	}
}
