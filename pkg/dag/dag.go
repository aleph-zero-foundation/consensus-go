// Package dag implements a basic dag that accepts any sequence of units.
package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type dag struct {
	nProcesses uint16
	units      *unitBag
	primeUnits *levelMap
	maxUnits   gomel.SlottedUnits
}

// New constructs a dag for a given number of processes.
func New(n uint16) gomel.Dag {
	return &dag{
		nProcesses: n,
		units:      newUnitBag(),
		primeUnits: newLevelMap(n, 10),
		maxUnits:   newSlottedUnits(n),
	}
}

// IsQuorum checks if the given number of processes forms a quorum amongst all processes.
func (dag *dag) IsQuorum(number uint16) bool {
	return gomel.IsQuorum(dag.nProcesses, number)
}

// NProc returns the number of processes which use the dag.
func (dag *dag) NProc() uint16 {
	return dag.nProcesses
}

// PrimeUnits returns the prime units at the requested level, indexed by their creator ids.
func (dag *dag) PrimeUnits(level int) gomel.SlottedUnits {
	res, err := dag.primeUnits.getLevel(level)
	if err != nil {
		return newSlottedUnits(dag.nProcesses)
	}
	return res
}

// MaximalUnitsPerProcess returns the maximal units created by respective processes.
func (dag *dag) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return dag.maxUnits
}

// Get returns a slice of units corresponding to the hashes provided.
// If a unit of a given hash is not present in the dag, the value at the same index in the result is nil.
func (dag *dag) Get(hashes []*gomel.Hash) []gomel.Unit {
	result, _ := dag.units.get(hashes)
	return result
}
