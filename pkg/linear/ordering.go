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

// getAntichainLayers for a given timing unit tu, returns all the units in its timing round
// divided into layers.
// 0-th layer is formed by minimal units in this timing round
// 1-st layer is formed by minimal units when the 0th layer is removed
// etc.
func (o *ordering) getAntichainLayers(tu gomel.Unit) [][]gomel.Unit {
	unitToLayer := make(map[gomel.Hash]int)
	seenUnits := make(map[gomel.Hash]bool)
	result := [][]gomel.Unit{}

	var dfs func(u gomel.Unit)
	dfs = func(u gomel.Unit) {
		seenUnits[*u.Hash()] = true
		minLayerBelow := -1
		for _, uParent := range u.Parents() {
			if _, ok := o.unitPositionInOrder[*uParent.Hash()]; ok {
				// uParent was already ordered and doesn't belong to this timing round
				continue
			}
			if !seenUnits[*uParent.Hash()] {
				dfs(uParent)
			}
			if unitToLayer[*uParent.Hash()] > minLayerBelow {
				minLayerBelow = unitToLayer[*uParent.Hash()]
			}
		}
		uLayer := minLayerBelow + 1
		unitToLayer[*u.Hash()] = uLayer
		if len(result) <= uLayer {
			result = append(result, []gomel.Unit{u})
		} else {
			result[uLayer] = append(result[uLayer], u)
		}
	}
	dfs(tu)
	return result
}

// sortTimingRound is sorting units in a timing round
func sortTimingRound(units []gomel.Unit, layers [][]gomel.Unit) {
	var totalXOR gomel.Hash
	for _, u := range units {
		totalXOR.XOREqual(u.Hash())
	}
	// tiebreaker is a map from units to its tiebreaker value
	tiebreaker := make(map[gomel.Hash]*gomel.Hash)
	for _, u := range units {
		tiebreaker[*u.Hash()] = gomel.XOR(&totalXOR, u.Hash())
	}

	unitLayer := make(map[gomel.Hash]int)
	for i := range layers {
		for _, u := range layers[i] {
			unitLayer[*u.Hash()] = i
		}
	}

	// break_ties from paper is equivalent to lexicographic sort by
	// (unitLayer[u], tiebreaker[u])
	sort.Slice(units, func(i, j int) bool {
		dhi := unitLayer[*units[i].Hash()]
		dhj := unitLayer[*units[j].Hash()]
		if dhi != dhj {
			return dhi < dhj
		}
		tbi := tiebreaker[*units[i].Hash()]
		tbj := tiebreaker[*units[j].Hash()]
		return tbi.LessThan(tbj)
	})
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

	layers := o.getAntichainLayers(timingUnit)
	unitsToOrder := []gomel.Unit{}
	for i := range layers {
		unitsToOrder = append(unitsToOrder, layers[i]...)
	}
	sortTimingRound(unitsToOrder, layers)

	// updating orderedUnits, unitPositionInOrder
	nAlreadyOrdered := len(o.unitPositionInOrder)
	for i, u := range unitsToOrder {
		o.orderedUnits = append(o.orderedUnits, u)
		o.unitPositionInOrder[*u.Hash()] = nAlreadyOrdered + i
	}

	return unitsToOrder
}
