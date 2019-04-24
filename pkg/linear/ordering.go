package linear

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"sort"
)

// Ordering is an implementation of LinearOrdering intended to work with a growing Poset.
type Ordering struct {
	poset       gomel.Poset
	timingUnits []gomel.Unit
	crp         gomel.CommonRandomPermutation
}

// NewOrdering creates a ordering wrapper for the given poset.
func NewOrdering(poset gomel.Poset, crp gomel.CommonRandomPermutation) gomel.LinearOrdering {
	return &Ordering{
		poset:       poset,
		timingUnits: []gomel.Unit{},
		crp:         crp,
	}
}

// Returns maximal level of a unit in a poset
func posetMaxLevel(p gomel.Poset) int {
	maxLevel := -1
	p.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if v.Level() > maxLevel {
				maxLevel = v.Level()
			}
		}
		return true
	})
	return maxLevel
}

// AttemptTimingDecision picks as many timing units as possible and returns the level up to which the timing units are picked.
func (o *Ordering) AttemptTimingDecision() int {
	maxLevel := posetMaxLevel(o.poset)
	for level := len(o.timingUnits); level <= maxLevel; level++ {
		u := o.DecideTimingOnLevel(level)
		if u != nil {
			o.timingUnits = append(o.timingUnits, u)
		} else {
			return level
		}
	}
	return 0
}

// Tries to pick a timing unit on a given level
// returns nil if it cannot be decided yet
func (o *Ordering) DecideTimingOnLevel(level int) gomel.Unit {
	VOTING_LEVEL := 3 // TODO: Read from config
	if posetMaxLevel(o.poset) < level+VOTING_LEVEL {
		return nil
	}
	for _, pid := range o.crp.Get(level) {
		primeUnitsByCurrProcess := o.poset.PrimeUnits(level).Get(pid)
		sort.Slice(primeUnitsByCurrProcess, func(i, j int) bool {
			return primeUnitsByCurrProcess[i].Hash().LessThan(primeUnitsByCurrProcess[j].Hash())
		})
		for _, uc := range primeUnitsByCurrProcess {
			decision := decideUnitIsPopular(o.poset, uc)
			if decision == POPULAR {
				return uc
			}
			if decision == UNDECIDED {
				return nil
			}
		}
	}
	return nil
}

// TimingRound returns all the units in timing round r. If the timing decision has not yet been taken it returns an error.
func (o *Ordering) TimingRound(r int) ([]gomel.Unit, error) {
	// TODO: implement
	return nil, nil
}
