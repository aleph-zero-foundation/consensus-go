package dag

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type slottedUnits struct {
	contents [][]gomel.Unit
	mxs      []sync.RWMutex
}

func newSlottedUnits(n uint16) *slottedUnits {
	return &slottedUnits{
		contents: make([][]gomel.Unit, n),
		mxs:      make([]sync.RWMutex, n),
	}
}

// Get returns the units at the provided id.
// MODIFYING THE RETURNED VALUE DIRECTLY RESULTS IN UNDEFINED BEHAVIOUR!
func (su *slottedUnits) Get(id uint16) []gomel.Unit {
	if int(id) >= len(su.mxs) {
		return []gomel.Unit{}
	}
	su.mxs[id].RLock()
	defer su.mxs[id].RUnlock()
	result := su.contents[id]
	return result
}

// Set replaces the units at the provided id with units.
func (su *slottedUnits) Set(id uint16, units []gomel.Unit) {
	if int(id) >= len(su.mxs) {
		return
	}
	su.mxs[id].Lock()
	defer su.mxs[id].Unlock()
	su.contents[id] = units
}

// Iterate runs work on its contents consecutively, until it returns false or the contents run out.
func (su *slottedUnits) Iterate(work func(units []gomel.Unit) bool) {
	for id := range su.mxs {
		if !work(su.Get(uint16(id))) {
			return
		}
	}
}
