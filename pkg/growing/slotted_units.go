package growing

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type slottedUnits struct {
	contents [][]gomel.Unit
	mxs      []sync.RWMutex
}

func newSlottedUnits(n int) *slottedUnits {
	return &slottedUnits{
		contents: make([][]gomel.Unit, n),
		mxs:      make([]sync.RWMutex, n),
	}
}

// Get returns the units at the provided id.
// MODIFYING THE RETURNED VALUE DIRECTLY RESULTS IN UNDEFINED BEHAVIOUR!
func (su *slottedUnits) Get(id int) []gomel.Unit {
	if id < 0 || id >= len(su.mxs) {
		return []gomel.Unit{}
	}
	su.mxs[id].RLock()
	defer su.mxs[id].RUnlock()
	result := su.contents[id]
	return result
}

// Set replaces the units at the provided id with units.
func (su *slottedUnits) Set(id int, units []gomel.Unit) {
	if id < 0 || id >= len(su.mxs) {
		return
	}
	su.mxs[id].Lock()
	defer su.mxs[id].Unlock()
	su.contents[id] = units
}

// Iterate runs work on its contents cosecutively, until it returns false or the contents run out.
func (su *slottedUnits) Iterate(work func(units []gomel.Unit) bool) {
	for id := 0; id < len(su.mxs); id++ {
		if !work(su.Get(id)) {
			return
		}
	}
}
