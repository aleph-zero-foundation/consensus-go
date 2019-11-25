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
	lastDecideResult bool
	orderStartLevel  int
	crpFixedPrefix   uint16
	dag              gomel.Dag
	randomSource     gomel.RandomSource
	currentTU        gomel.Unit
	lastTUs          []gomel.Unit
	decider          *superMajorityDecider
	log              zerolog.Logger
}

// NewOrdering creates an Ordering wrapper around a given dag.
func NewOrdering(dag gomel.Dag, rs gomel.RandomSource, orderStartLevel int, crpFixedPrefix uint16, log zerolog.Logger) gomel.LinearOrdering {

	stdDecider := newSuperMajorityDecider(dag, rs)

	return &ordering{
		dag:             dag,
		randomSource:    rs,
		orderStartLevel: orderStartLevel,
		crpFixedPrefix:  crpFixedPrefix,
		lastTUs:         make([]gomel.Unit, stdDecider.firstRoundZeroForCommonVote),
		decider:         stdDecider,
		log:             log,
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
	if o.lastDecideResult {
		o.lastDecideResult = false
		o.decider = newSuperMajorityDecider(o.dag, o.randomSource)
	}

	dagMaxLevel := dagMaxLevel(o.dag)
	if dagMaxLevel < o.orderStartLevel {
		return nil
	}

	level := o.orderStartLevel
	if o.currentTU != nil {
		level = o.currentTU.Level() + 1
	}
	if dagMaxLevel < level+firstDecidingRound {
		return nil
	}

	previousTU := o.currentTU
	var result gomel.Unit
	o.crpIterate(level, previousTU, func(uc gomel.Unit) bool {
		decision, decidedOn := o.decider.decideUnitIsPopular(uc, dagMaxLevel)
		if decision == popular {
			o.log.Info().Int(logging.Height, decidedOn).Int(logging.Size, dagMaxLevel).Int(logging.Round, level).Msg(logging.NewTimingUnit)

			o.lastTUs = append(o.lastTUs[:0], o.lastTUs[1:]...)
			o.lastTUs = append(o.lastTUs, o.currentTU)
			o.currentTU = uc
			o.decider = nil
			o.lastDecideResult = true

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
func (o *ordering) getAntichainLayers(tu gomel.Unit, prevTUs []gomel.Unit) [][]gomel.Unit {
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
			// check if it was already processed
			// NOTE we can prove that comparing with last k timing units, where k is the first round for which the deterministic
			// common vote is zero, is enough to verify if a unit was already ordered. Since the common vote for round k is 0,
			// every unit on level tu.Level()+k must be above a timing unit tu, otherwise some unit would decide 0 for it.
			if prevTU := prevTUs[len(prevTUs)-1]; prevTU != nil &&
				uParent.Level() <= prevTU.Level() {

				found := false
				for it := len(prevTUs) - 1; it >= 0; it-- {
					if gomel.Above(prevTUs[it], uParent) {
						found = true
						break
					}
				}
				if found {
					continue
				}
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

// TimingRound establishes the linear ordering on the units in the currently decided timing round and returns them.
// If the timing decision has not yet been taken it returns nil.
func (o *ordering) TimingRound() []gomel.Unit {
	if !o.lastDecideResult {
		return nil
	}

	layers := o.getAntichainLayers(o.currentTU, o.lastTUs)
	sortedUnits := mergeLayers(layers)
	return sortedUnits
}
