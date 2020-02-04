// Package dag implements a basic dag that accepts any sequence of units.
package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type dag struct {
	nProcesses  uint16
	epochID     gomel.EpochID
	units       *unitBag
	primeUnits  *fiberMap
	heightUnits *fiberMap
	maxUnits    gomel.SlottedUnits
	checks      []gomel.UnitChecker
	transforms  []gomel.UnitTransformer
	preInsert   []gomel.InsertHook
	postInsert  []gomel.InsertHook
}

// New constructs a dag for a given number of processes.
func New(n uint16) gomel.Dag {
	return &dag{
		nProcesses:  n,
		epochID:     gomel.EpochID(0),
		units:       newUnitBag(),
		primeUnits:  newFiberMap(n, 10),
		heightUnits: newFiberMap(n, 10),
		maxUnits:    newSlottedUnits(n),
	}
}

func (dag *dag) EpochID() gomel.EpochID {
	return dag.epochID
}

// IsQuorum checks if the given number of processes forms a quorum amongst all processes.
func (dag *dag) IsQuorum(number uint16) bool {
	return number >= gomel.MinimalQuorum(dag.nProcesses)
}

// NProc returns the number of processes which use the dag.
func (dag *dag) NProc() uint16 {
	return dag.nProcesses
}

// PrimeUnits returns the prime units at the requested level, indexed by their creator ids.
func (dag *dag) PrimeUnits(level int) gomel.SlottedUnits {
	res, err := dag.primeUnits.getFiber(level)
	if err != nil {
		return newSlottedUnits(dag.nProcesses)
	}
	return res
}

// UnitsOnHeight returns the units at the requested height, indexed by their creator ids.
func (dag *dag) UnitsOnHeight(height int) gomel.SlottedUnits {
	res, err := dag.heightUnits.getFiber(height)
	if err != nil {
		return newSlottedUnits(dag.nProcesses)
	}
	return res
}

// MaximalUnitsPerProcess returns the maximal units created by respective processes.
func (dag *dag) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return dag.maxUnits
}

func (dag *dag) GetUnit(hash *gomel.Hash) gomel.Unit {
	return dag.units.getOne(hash)
}

// GetUnits returns a slice of units corresponding to the hashes provided.
// If a unit of a given hash is not present in the dag, the corresponding value is nil.
// Returned int is the number of such missing units.
func (dag *dag) GetUnits(hashes []*gomel.Hash) []gomel.Unit {
	us, _ := dag.units.getMany(hashes)
	return us
}

func (dag *dag) GetByID(id uint64) []gomel.Unit {
	height, creator, epoch := gomel.DecodeID(id)
	if epoch != dag.EpochID() {
		return nil
	}
	fiber, err := dag.heightUnits.getFiber(height)
	if err != nil {
		return nil
	}
	return fiber.Get(creator)
}
