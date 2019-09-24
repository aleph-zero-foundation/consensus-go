// Package creating contains functions responsible for creating new units.
// It also contains a publicly available implementation of a preunit.
package creating

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type noAvailableParents struct{}

func (e *noAvailableParents) Error() string {
	return "No legal parents for the unit."
}

// getPredecessor picks one of the units in mu produced by the given creator.
func getPredecessor(mu gomel.SlottedUnits, creator uint16) gomel.Unit {
	maxUnits := mu.Get(creator)
	if len(maxUnits) == 0 {
		return nil
	}
	return maxUnits[0]
}

// newDealingUnit creates a new preunit with the given creator and no parents.
func newDealingUnit(creator, NProc uint16, data []byte, rs gomel.RandomSource) gomel.Preunit {
	rsData, _ := rs.DataToInclude(creator, nil, 0)
	return NewPreunit(creator, make([]*gomel.Hash, NProc), data, rsData)
}

func makeConsistent(parents []gomel.Unit) {
	for i := 0; i < len(parents); i++ {
		for j := 0; j < len(parents); j++ {
			if parents[j] == nil {
				continue
			}
			u := parents[j].Parents()[i]
			if parents[i] == nil || parents[i].Below(u) {
				parents[i] = u
			}
		}
	}
}

func ancestorBelowLevel(u gomel.Unit, level int) gomel.Unit {
	for u != nil && u.Level() > level {
		u = gomel.Predecessor(u)
	}
	return u
}

func hashes(units []gomel.Unit) []*gomel.Hash {
	result := make([]*gomel.Hash, len(units))
	for i, u := range units {
		if u == nil {
			result[i] = nil
		} else {
			result[i] = u.Hash()
		}
	}
	return result
}

func levelFromParents(dag gomel.Dag, parents []gomel.Unit) int {
	level := 0
	onLevel := uint16(0)
	for i := uint16(0); i < dag.NProc(); i++ {
		if parents[i] == nil {
			continue
		}
		if parents[i].Level() == level {
			onLevel++
		} else if parents[i].Level() > level {
			onLevel = 1
			level = parents[i].Level()
		}
	}
	if gomel.IsQuorum(dag.NProc(), onLevel) {
		level++
	}
	return level
}

func pickParents(dag gomel.Dag, mu gomel.SlottedUnits, predecessor gomel.Unit, canSkipLevel bool) []gomel.Unit {
	creator := predecessor.Creator()
	parents := make([]gomel.Unit, dag.NProc())
	parents[creator] = predecessor

	for i := uint16(0); i < dag.NProc(); i++ {
		if i == creator {
			continue
		}
		units := mu.Get(i)
		if len(units) == 0 {
			parents[i] = nil
		} else {
			// If there is a fork we should choose the one we have committed to.
			// For now just taking the first option.
			candidate := units[0]
			if !canSkipLevel {
				candidate = ancestorBelowLevel(candidate, predecessor.Level())
			}
			parents[i] = candidate
		}
	}
	makeConsistent(parents)
	return parents
}

// NewUnit creates a preunit for a given process.
func NewUnit(dag gomel.Dag, creator uint16, data []byte, rs gomel.RandomSource, canSkipLevel bool) (gomel.Preunit, int, error) {
	mu := dag.MaximalUnitsPerProcess()
	predecessor := getPredecessor(mu, creator)
	// This is the first unit creator is creating, so it should be a dealing unit.
	if predecessor == nil {
		return newDealingUnit(creator, dag.NProc(), data, rs), 0, nil
	}

	parents := pickParents(dag, mu, predecessor, canSkipLevel)
	level := levelFromParents(dag, parents)
	// We require each unit to be a prime unit.
	if level == predecessor.Level() {
		return nil, 0, &noAvailableParents{}
	}

	rsData, err := rs.DataToInclude(creator, parents, level)
	if err != nil {
		return nil, 0, err
	}
	return NewPreunit(creator, hashes(parents), data, rsData), level, nil
}
