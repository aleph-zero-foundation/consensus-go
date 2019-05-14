package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"sync"
)

type slottedUnits struct {
	sync.RWMutex
	contents [][]gomel.Unit
}

func (su *slottedUnits) Get(id int) []gomel.Unit {
	su.RLock()
	defer su.RUnlock()
	return su.contents[id]
}

func (su *slottedUnits) Set(id int, units []gomel.Unit) {
	su.Lock()
	defer su.Unlock()
	su.contents[id] = units
}

func (su *slottedUnits) Iterate(work func([]gomel.Unit) bool) {
	su.RLock()
	defer su.RUnlock()
	for _, units := range su.contents {
		if !work(units) {
			return
		}
	}
}

func newSlottedUnits(n int) gomel.SlottedUnits {
	return &slottedUnits{
		contents: make([][]gomel.Unit, n),
	}
}
