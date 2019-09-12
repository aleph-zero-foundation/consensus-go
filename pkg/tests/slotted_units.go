package tests

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type slottedUnits struct {
	sync.RWMutex
	contents [][]gomel.Unit
}

func (su *slottedUnits) Get(id uint16) []gomel.Unit {
	su.RLock()
	defer su.RUnlock()
	return su.contents[id]
}

func (su *slottedUnits) Set(id uint16, units []gomel.Unit) {
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

func newSlottedUnits(n uint16) gomel.SlottedUnits {
	return &slottedUnits{
		contents: make([][]gomel.Unit, n),
	}
}
