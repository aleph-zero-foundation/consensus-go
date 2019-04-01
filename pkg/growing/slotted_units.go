package growing

import (
	"sync"

	a "gitlab.com/alephledger/consensus-go/pkg"
)

type slottedUnits struct {
	contents [][]a.Unit
	mxs      []sync.RWMutex
}

func newSlottedUnits(n int) *slottedUnits {
	return &slottedUnits{
		contents: make([][]a.Unit, n),
		mxs:      make([]sync.RWMutex, n),
	}
}

// Returns a copy of the units at the provided id.
func (su *slottedUnits) Get(id int) []a.Unit {
	if id < 0 || id > len(su.mxs) {
		return []a.Unit{}
	}
	su.mxs[id].RLock()
	defer su.mxs[id].RUnlock()
	result := make([]a.Unit, len(su.contents[id]))
	copy(result, su.contents[id])
	return result
}

// Replaces the units at the provided id with units.
func (su *slottedUnits) Set(id int, units []a.Unit) {
	if id < 0 || id > len(su.mxs) {
		return
	}
	su.mxs[id].Lock()
	defer su.mxs[id].Unlock()
	su.contents[id] = make([]a.Unit, len(units))
	copy(su.contents[id], units)
}
