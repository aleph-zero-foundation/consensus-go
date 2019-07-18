package fallback

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
)

type fetchFallback struct {
	dag      gomel.Dag
	requests chan<- fetch.Request
}

func (f *fetchFallback) Run(pu gomel.Preunit) {
	hashes := pu.Parents()
	parents := f.dag.Get(hashes)
	toRequest := []*gomel.Hash{}
	for i, h := range hashes {
		if parents[i] == nil {
			toRequest = append(toRequest, h)
		}
	}
	if len(toRequest) > 0 {
		f.requests <- fetch.Request{
			Pid:    uint16(pu.Creator()),
			Hashes: toRequest,
		}
	}
}

// NewFetch creates a fallback that pushes fetch requests for unknown parents to the provided channel.
func NewFetch(dag gomel.Dag, requests chan<- fetch.Request) sync.Fallback {
	return &fetchFallback{
		dag:      dag,
		requests: requests,
	}
}
