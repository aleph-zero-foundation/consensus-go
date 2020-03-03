package linear

import (
	"sort"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type timingRound struct {
	currentTU gomel.Unit
	lastTUs   []gomel.Unit
}

func newTimingRound(currentTimingUnit gomel.Unit, lastTimingUnits []gomel.Unit) *timingRound {
	return &timingRound{currentTU: currentTimingUnit, lastTUs: lastTimingUnits}
}

func (tr *timingRound) OrderedUnits() []gomel.Unit {
	layers := getAntichainLayers(tr.currentTU, tr.lastTUs)
	sortedUnits := mergeLayers(layers)
	return sortedUnits
}

// NOTE we can prove that comparing with last k timing units, where k is the first round for which the deterministic
// common vote is zero, is enough to verify if a unit was already ordered. Since the common vote for round k is 0,
// every unit on level tu.Level()+k must be above a timing unit tu, otherwise some unit would decide 0 for it.
func checkIfAlreadyOrdered(u gomel.Unit, prevTUs []gomel.Unit) bool {
	if prevTU := prevTUs[len(prevTUs)-1]; prevTU == nil || u.Level() > prevTU.Level() {
		return false
	}
	for it := len(prevTUs) - 1; it >= 0; it-- {
		if gomel.Above(prevTUs[it], u) {
			return true
		}
	}
	return false
}

// getAntichainLayers for a given timing unit tu, returns all the units in its timing round
// divided into layers.
// 0-th layer is formed by minimal units in this timing round.
// 1-st layer is formed by minimal units when the 0th layer is removed.
// etc.
func getAntichainLayers(tu gomel.Unit, prevTUs []gomel.Unit) [][]gomel.Unit {
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
			if checkIfAlreadyOrdered(uParent, prevTUs) {
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
