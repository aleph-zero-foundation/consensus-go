package linear

import (
	"sort"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Ordering is an implementation of LinearOrdering interface.
type ordering struct {
	poset               gomel.Poset
	timingUnits         *safeUnitSlice
	crp                 CommonRandomPermutation
	unitPositionInOrder map[gomel.Hash]int
	orderedUnits        []gomel.Unit
}

// NewOrdering creates an Ordering wrapper around a given poset.
func NewOrdering(poset gomel.Poset, crp CommonRandomPermutation) gomel.LinearOrdering {
	return &ordering{
		poset:               poset,
		timingUnits:         newSafeUnitSlice(),
		crp:                 crp,
		unitPositionInOrder: make(map[gomel.Hash]int),
		orderedUnits:        []gomel.Unit{},
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
func (o *ordering) AttemptTimingDecision() int {
	maxLevel := posetMaxLevel(o.poset)
	for level := o.timingUnits.safeLen(); level <= maxLevel; level++ {
		u := o.DecideTimingOnLevel(level)
		if u != nil {
			o.timingUnits.safeAppend(u)
		} else {
			return level
		}
	}
	return o.timingUnits.safeLen()
}

// DecideTimingOnLevel tries to pick a timing unit on a given level. Returns nil if it cannot be decided yet.
func (o *ordering) DecideTimingOnLevel(level int) gomel.Unit {
	if posetMaxLevel(o.poset) < level+votingLevel {
		return nil
	}
	for _, pid := range o.crp.Get(level) {
		primeUnitsByCurrProcess := o.poset.PrimeUnits(level).Get(pid)
		sort.Slice(primeUnitsByCurrProcess, func(i, j int) bool {
			return primeUnitsByCurrProcess[i].Hash().LessThan(primeUnitsByCurrProcess[j].Hash())
		})
		for _, uc := range primeUnitsByCurrProcess {
			decision := decideUnitIsPopular(o.poset, uc)
			if decision == popular {
				return uc
			}
			if decision == undecided {
				return nil
			}
		}
	}
	return nil
}

// TimingRound returns all the units in timing round r. If the timing decision has not yet been taken it returns an error.
func (o *ordering) TimingRound(r int) ([]gomel.Unit, error) {
	if o.timingUnits.safeLen() <= r {
		return nil, gomel.NewOrderingError("Timing decision has not yet been taken on this level")
	}
	timingUnit := o.timingUnits.safeGet(r)

	// If we already ordered this unit we can read the answer from orderedUnits
	if roundEnds, alreadyOrdered := o.unitPositionInOrder[*timingUnit.Hash()]; alreadyOrdered {
		roundBegins := 0
		if r != 0 {
			roundBegins = o.unitPositionInOrder[*o.timingUnits.safeGet(r - 1).Hash()] + 1
		}
		return o.orderedUnits[roundBegins:(roundEnds + 1)], nil
	}

	var totalXOR gomel.Hash

	// dependencyHeight of a unit u in a given set of units is
	// 0 --- when u is minimal
	// max(dependencyHeight(u.Parents)) + 1 --- otherwise
	dependencyHeight := make(map[gomel.Hash]int)

	// the following dfs
	// (1) collects all units for this timing round
	// (2) calculates dependencyHeight for those units
	// (3) calculates totalXOR
	seenUnits := make(map[gomel.Hash]bool)
	unitsToOrder := []gomel.Unit{}

	var dfs func(u gomel.Unit)
	dfs = func(u gomel.Unit) {
		totalXOR.XOREqual(u.Hash())
		seenUnits[*u.Hash()] = true
		unitsToOrder = append(unitsToOrder, u)
		minDependencyHeightBelow := -1
		for _, uParent := range u.Parents() {
			if _, ok := o.unitPositionInOrder[*uParent.Hash()]; ok {
				continue
			}
			if _, ok := seenUnits[*uParent.Hash()]; !ok {
				dfs(uParent)
			}
			if dependencyHeight[*uParent.Hash()] > minDependencyHeightBelow {
				minDependencyHeightBelow = dependencyHeight[*uParent.Hash()]
			}
		}
		dependencyHeight[*u.Hash()] = minDependencyHeightBelow + 1
	}
	dfs(timingUnit)

	// tiebreaker is a map from units to its tiebreaker value
	tiebreaker := make(map[gomel.Hash]*gomel.Hash)
	for _, u := range unitsToOrder {
		tiebreaker[*u.Hash()] = gomel.XOR(&totalXOR, u.Hash())
	}

	// break_ties from paper is equivalent to lexicographic sort by
	// (dependencyHeight[u], tiebreaker[u])
	sort.Slice(unitsToOrder, func(i, j int) bool {
		dhi := dependencyHeight[*unitsToOrder[i].Hash()]
		dhj := dependencyHeight[*unitsToOrder[j].Hash()]
		if dhi != dhj {
			return dhi < dhj
		}
		tbi := tiebreaker[*unitsToOrder[i].Hash()]
		tbj := tiebreaker[*unitsToOrder[j].Hash()]
		return tbi.LessThan(tbj)
	})

	// updating orderedUnits, unitPositionInOrder
	nAlreadyOrdered := len(o.unitPositionInOrder)
	for i, u := range unitsToOrder {
		o.orderedUnits = append(o.orderedUnits, u)
		o.unitPositionInOrder[*u.Hash()] = nAlreadyOrdered + i
	}

	return unitsToOrder, nil
}
