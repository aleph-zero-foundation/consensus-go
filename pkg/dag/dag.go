// Package dag implements a basic dag that accepts any sequence of units.
package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type dag struct {
	nProc       uint16
	epochID     gomel.EpochID
	units       *unitBag
	levelUnits  *fiberMap
	heightUnits *fiberMap
	maxUnits    gomel.SlottedUnits
	checks      []gomel.UnitChecker
	preInsert   []gomel.InsertHook
	postInsert  []gomel.InsertHook
}

// New constructs a dag for a given number of processes.
func New(conf config.Config, epochID gomel.EpochID) gomel.Dag {
	return &dag{
		nProc:       conf.NProc,
		epochID:     epochID,
		units:       newUnitBag(),
		levelUnits:  newFiberMap(conf.NProc, 10),
		heightUnits: newFiberMap(conf.NProc, 10),
		maxUnits:    newSlottedUnits(conf.NProc),
		checks:      append([]gomel.UnitChecker(nil), conf.Checks...),
	}
}

func (dag *dag) AddCheck(check gomel.UnitChecker) {
	dag.checks = append(dag.checks, check)
}

func (dag *dag) BeforeInsert(hook gomel.InsertHook) {
	dag.preInsert = append(dag.preInsert, hook)
}

func (dag *dag) AfterInsert(hook gomel.InsertHook) {
	dag.postInsert = append(dag.postInsert, hook)
}

func (dag *dag) EpochID() gomel.EpochID {
	return dag.epochID
}

// IsQuorum checks if the given number of processes forms a quorum amongst all processes.
func (dag *dag) IsQuorum(number uint16) bool {
	return number >= gomel.MinimalQuorum(dag.nProc)
}

// NProc returns the number of processes which use the dag.
func (dag *dag) NProc() uint16 {
	return dag.nProc
}

// UnitsOnLevel returns the prime units at the requested level, indexed by their creator ids.
func (dag *dag) UnitsOnLevel(level int) gomel.SlottedUnits {
	res, err := dag.levelUnits.getFiber(level)
	if err != nil {
		return newSlottedUnits(dag.nProc)
	}
	return res
}

// UnitsAbove returns all units present in dag that are above (in height sense) given heights.
// When called with nil argument, returns all units in the dag.
// Units returned by this method are in random order.
func (dag *dag) UnitsAbove(heights []int) []gomel.Unit {
	if heights == nil {
		return dag.units.getAll()
	}
	return dag.heightUnits.above(heights)
}

// MaximalUnitsPerProcess returns the maximal units created by respective processes.
func (dag *dag) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return dag.maxUnits
}

// GetUnit returns a unit with the given hash, if present in dag.
func (dag *dag) GetUnit(hash *gomel.Hash) gomel.Unit {
	return dag.units.getOne(hash)
}

// GetUnits returns a slice of units corresponding to the hashes provided.
// If a unit of a given hash is not present in the dag, the corresponding value is nil.
func (dag *dag) GetUnits(hashes []*gomel.Hash) []gomel.Unit {
	return dag.units.getMany(hashes)
}

// GetByID returns all units in dag with the given ID. There is more than one only in case of forks.
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
