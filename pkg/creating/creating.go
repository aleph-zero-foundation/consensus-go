// Package creating contains functions responsible for creating new units.
//
// It also contains a publicly available implementation of a preunit.
//
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
	rsData, _ := rs.DataToInclude(creator, make([]gomel.Unit, NProc), 0)
	return NewPreunit(creator, gomel.EmptyCrown(NProc), data, rsData)
}

func makeConsistent(parents []gomel.Unit) {
	for i := 0; i < len(parents); i++ {
		for j := 0; j < len(parents); j++ {
			if parents[j] == nil {
				continue
			}
			u := parents[j].Parents()[i]
			if parents[i] == nil || (u != nil && u.Level() > parents[i].Level()) {
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
			// If there is a fork we are choosing the parent with highest possible level.
			candidate := units[0]
			for _, fork := range units {
				if fork.Level() > candidate.Level() {
					candidate = fork
				}
			}
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
// If this is the first unit by the creator it returns a dealing unit.
// Otherwise for each process it chooses maximal unit with the highest level created by the process,
// and uses those units as parents of the newly created unit.
// If canSkipLevel flag isn't set, we are considering only parents of level at most predecessor.Level().
func NewUnit(dag gomel.Dag, creator uint16, data []byte, rs gomel.RandomSource, canSkipLevel bool) (gomel.Preunit, int, error) {
	mu := dag.MaximalUnitsPerProcess()
	predecessor := getPredecessor(mu, creator)
	// This is the first unit creator is creating, so it should be a dealing unit.
	if predecessor == nil {
		return newDealingUnit(creator, dag.NProc(), data, rs), 0, nil
	}

	parents := pickParents(dag, mu, predecessor, canSkipLevel)
	level := gomel.LevelFromParents(parents)
	// We require each unit to be a prime unit.
	if level == predecessor.Level() {
		return nil, 0, &noAvailableParents{}
	}

	rsData, err := rs.DataToInclude(creator, parents, level)
	if err != nil {
		return nil, 0, err
	}
	return NewPreunit(creator, gomel.CrownFromParents(parents), data, rsData), level, nil
}
