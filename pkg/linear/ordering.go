// Package linear implements the algorithm for deciding the linear order for units in a dag.
package linear

import (
	"sort"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Ordering is an implementation of LinearOrdering interface.
type ordering struct {
	dag                 gomel.Dag
	randomSource        gomel.RandomSource
	timingUnits         *safeUnitSlice
	unitPositionInOrder map[gomel.Hash]int
	orderedUnits        []gomel.Unit
	orderStartLevel     int
	crpFixedPrefix      uint16
	decider             *superMajorityDecider
	log                 zerolog.Logger
}

// NewOrdering creates an Ordering wrapper around a given dag.
func NewOrdering(dag gomel.Dag, rs gomel.RandomSource, orderStartLevel int, crpFixedPrefix uint16, log zerolog.Logger) gomel.LinearOrdering {

	stdDecider := newSuperMajorityDecider(dag, rs)

	return &ordering{
		dag:                 dag,
		randomSource:        rs,
		timingUnits:         newSafeUnitSlice(orderStartLevel),
		unitPositionInOrder: make(map[gomel.Hash]int),
		orderedUnits:        []gomel.Unit{},
		orderStartLevel:     orderStartLevel,
		crpFixedPrefix:      crpFixedPrefix,
		decider:             stdDecider,
		log:                 log,
	}
}

// dagMaxLevel returns the maximal level of a unit in the dag.
func dagMaxLevel(dag gomel.Dag) int {
	maxLevel := -1
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if v.Level() > maxLevel {
				maxLevel = v.Level()
			}
		}
		return true
	})
	return maxLevel
}

// DecideTiming tries to pick the next timing unit. Returns nil if it cannot be decided yet.
func (o *ordering) DecideTiming() gomel.Unit {
	level := o.timingUnits.length()

	dagMaxLevel := dagMaxLevel(o.dag)
	if dagMaxLevel < level+firstDecidingRound {
		return nil
	}

	var previousTU gomel.Unit
	if level > o.orderStartLevel {
		previousTU = o.timingUnits.get(level - 1)
	}

	var result gomel.Unit
	o.crpIterate(level, previousTU, func(uc gomel.Unit) bool {
		decision, decidedOn := o.decider.decideUnitIsPopular(uc, dagMaxLevel)
		if decision == popular {
			o.log.Info().Int(logging.Height, decidedOn).Int(logging.Size, dagMaxLevel).Int(logging.Round, level).Msg(logging.NewTimingUnit)
			o.timingUnits.appendOrIgnore(level, uc)
			result = uc
			return false
		}
		if decision == undecided {
			return false
		}
		return true
	})
	return result
}

// getAntichainLayers for a given timing unit tu, returns all the units in its timing round
// divided into layers.
// 0-th layer is formed by minimal units in this timing round.
// 1-st layer is formed by minimal units when the 0th layer is removed.
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
			if uParent == nil {
				continue
			}
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

func mergeLayers(layers [][]gomel.Unit) []gomel.Unit {
	var totalXOR gomel.Hash
	for i := range layers {
		for _, u := range layers[i] {
			totalXOR.XOREqual(u.Hash())
		}
	}
	// tiebreaker is a map from units to its tiebreaker value
	tiebreaker := make(map[gomel.Hash]*gomel.Hash)
	for l := range layers {
		for _, u := range layers[l] {
			tiebreaker[*u.Hash()] = gomel.XOR(&totalXOR, u.Hash())
		}
	}

	sortedUnits := []gomel.Unit{}

	for l := range layers {
		sort.Slice(layers[l], func(i, j int) bool {
			tbi := tiebreaker[*layers[l][i].Hash()]
			tbj := tiebreaker[*layers[l][j].Hash()]
			return tbi.LessThan(tbj)
		})
		sortedUnits = append(sortedUnits, layers[l]...)
	}
	return sortedUnits
}

// TimingRound establishes the linear ordering on the units in the timing round r and returns them.
// If the timing decision has not yet been taken it returns nil.
func (o *ordering) TimingRound(r int) []gomel.Unit {
	if o.timingUnits.length() <= r {
		return nil
	}
	timingUnit := o.timingUnits.get(r)

	// If we already ordered this unit we can read the answer from orderedUnits
	if roundEnds, alreadyOrdered := o.unitPositionInOrder[*timingUnit.Hash()]; alreadyOrdered {
		roundBegins := 0
		if r != o.orderStartLevel {
			roundBegins = o.unitPositionInOrder[*o.timingUnits.get(r - 1).Hash()] + 1
		}
		return o.orderedUnits[roundBegins:(roundEnds + 1)]
	}

	layers := o.getAntichainLayers(timingUnit)
	sortedUnits := mergeLayers(layers)

	// updating orderedUnits, unitPositionInOrder
	nAlreadyOrdered := len(o.unitPositionInOrder)
	for i, u := range sortedUnits {
		o.orderedUnits = append(o.orderedUnits, u)
		o.unitPositionInOrder[*u.Hash()] = nAlreadyOrdered + i
	}

	return sortedUnits
}
