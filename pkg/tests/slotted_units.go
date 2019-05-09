package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type slottedUnits struct {
	contents [][]gomel.Unit
}

func (su *slottedUnits) Get(id int) []gomel.Unit {
	return su.contents[id]
}

func (su *slottedUnits) Set(id int, units []gomel.Unit) {
	su.contents[id] = units
}

func (su *slottedUnits) Iterate(work func([]gomel.Unit) bool) {
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
