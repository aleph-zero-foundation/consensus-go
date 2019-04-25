package linear

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"sort"
)

// Ordering is an implementation of LinearOrdering interface.
type Ordering struct {
	poset       gomel.Poset
	timingUnits []gomel.Unit
	crp         CommonRandomPermutation
}

// NewOrdering creates an Ordering wrapper around a given poset.
func NewOrdering(poset gomel.Poset, crp CommonRandomPermutation) gomel.LinearOrdering {
	return &Ordering{
		poset:       poset,
		timingUnits: []gomel.Unit{},
		crp:         crp,
	}
}

// posetMaxLevel returns the maximal level of a unit in a poset.
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

// AttemptTimingDecision chooses as many new timing units as possible and returns the level of the highest timing unit chosen so far.
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
	return len(o.timingUnits)
}

// DecideTimingOnLevel tries to pick a timing unit on a given level. Returns nil if it cannot be decided yet.
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

// TODO: implement
func (o *Ordering) TimingRound(r int) ([]gomel.Unit, error) {
	return nil, nil
}
