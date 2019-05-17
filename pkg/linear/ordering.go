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
	votingLevel         int
	piDeltaLevel        int
	proofMemo           map[[2]gomel.Hash]bool
	voteMemo            map[[2]gomel.Hash]vote
	piMemo              map[[2]gomel.Hash]vote
	deltaMemo           map[[2]gomel.Hash]vote
	decisionMemo        map[gomel.Hash]vote
}

// NewOrdering creates an Ordering wrapper around a given poset.
func NewOrdering(poset gomel.Poset, votingLevel int, PiDeltaLevel int) gomel.LinearOrdering {
	return &ordering{
		poset:               poset,
		timingUnits:         newSafeUnitSlice(),
		crp:                 NewCommonRandomPermutation(poset),
		unitPositionInOrder: make(map[gomel.Hash]int),
		orderedUnits:        []gomel.Unit{},
		votingLevel:         votingLevel,
		piDeltaLevel:        PiDeltaLevel,
		proofMemo:           make(map[[2]gomel.Hash]bool),
		voteMemo:            make(map[[2]gomel.Hash]vote),
		piMemo:              make(map[[2]gomel.Hash]vote),
		deltaMemo:           make(map[[2]gomel.Hash]vote),
		decisionMemo:        make(map[gomel.Hash]vote),
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

// DecideTimingOnLevel tries to pick a timing unit on a given level. Returns nil if it cannot be decided yet.
func (o *ordering) DecideTimingOnLevel(level int) gomel.Unit {
	// If we have already decided we can read the answer from memory
	if o.timingUnits.length() > level {
		return o.timingUnits.get(level)
	}

	if posetMaxLevel(o.poset) < level+o.votingLevel {
		return nil
	}
	for _, pid := range o.crp.Get(level) {
		primeUnitsByCurrProcess := o.poset.PrimeUnits(level).Get(pid)
		sort.Slice(primeUnitsByCurrProcess, func(i, j int) bool {
			return primeUnitsByCurrProcess[i].Hash().LessThan(primeUnitsByCurrProcess[j].Hash())
		})
		for _, uc := range primeUnitsByCurrProcess {
			decision := o.decideUnitIsPopular(uc)
			if decision == popular {
				o.timingUnits.pushBack(uc)
				return uc
			}
			if decision == undecided {
				return nil
			}
		}
	}
	return nil
}

// TimingRound returns all the units in timing round r. If the timing decision has not yet been taken it returns nil.
func (o *ordering) TimingRound(r int) []gomel.Unit {
	if o.timingUnits.length() <= r {
		return nil
	}
	timingUnit := o.timingUnits.get(r)

	// If we already ordered this unit we can read the answer from orderedUnits
	if roundEnds, alreadyOrdered := o.unitPositionInOrder[*timingUnit.Hash()]; alreadyOrdered {
		roundBegins := 0
		if r != 0 {
			roundBegins = o.unitPositionInOrder[*o.timingUnits.get(r - 1).Hash()] + 1
		}
		return o.orderedUnits[roundBegins:(roundEnds + 1)]
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
			if !seenUnits[*uParent.Hash()] {
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

	return unitsToOrder
}
