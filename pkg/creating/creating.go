// Package creating contains functions responsible for creating new units.
//
// It also contains a publicly available implementation of a preunit.
//
// All units created using functions in this package will have parents created by distinct processes
// and satisfying the expand primes rule. The first parent will also be the predecessor.
// Units created by processes known to be forking at the moment of creation will never be chosen as parents.
package creating

import (
	"math/rand"
	"sort"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type noAvailableParents struct{}

func (e *noAvailableParents) Error() string {
	return "No legal parents for the unit."
}

type visionSplit struct {
	visible   map[gomel.Unit]bool
	invisible map[gomel.Unit]bool
}

func newVisionSplit() *visionSplit {
	return &visionSplit{
		visible:   make(map[gomel.Unit]bool),
		invisible: make(map[gomel.Unit]bool),
	}
}

func (vs *visionSplit) newSeer(u gomel.Unit) bool {
	seesNew := false
	for nv := range vs.invisible {
		if vs.invisible[nv] && nv.Below(u) {
			vs.invisible[nv] = false
			vs.visible[nv] = true
			seesNew = true
		}
	}
	return seesNew
}

func (vs *visionSplit) hasQuorumIn(dag gomel.Dag) bool {
	return dag.IsQuorum(uint16(len(vs.visible)))
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
	return NewPreunit(creator, []*gomel.Hash{}, data, rsData)
}

// maxLevel returns the maximal level from units present in mu.
func maxLevel(mu gomel.SlottedUnits) int {
	result := -1
	mu.Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			if u.Level() > result {
				result = u.Level()
			}
		}
		return true
	})
	return result
}

// splitByBelow returns a split of the units in su into parts that are either visible or invisible to units.
func splitByBelow(su gomel.SlottedUnits, units []gomel.Unit) *visionSplit {
	result := newVisionSplit()
	su.Iterate(func(primes []gomel.Unit) bool {
		for _, prime := range primes {
			if gomel.BelowAny(prime, units) {
				result.visible[prime] = true
			} else {
				result.invisible[prime] = true
			}
		}
		return true
	})
	return result
}

// maximalIn checks if the given unit u is maximal (in the partial order sense) in units.
func maximalIn(u gomel.Unit, units []gomel.Unit) bool {
	for _, v := range units {
		if u != v && u.Below(v) {
			return false
		}
	}
	return true
}

// filterMaximal returns units that are maximal (in the partial order sense) from among units.
func filterMaximal(units []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	for _, u := range units {
		if maximalIn(u, units) {
			result = append(result, u)
		}
	}
	return result
}

// getCandidatesAtLevel chooses units from candidates that are:
// a) produced by non-forking process,
// b) at the given level,
// c) not below any unit in parents.
func getCandidatesAtLevel(candidates gomel.SlottedUnits, parents []gomel.Unit, level int) []gomel.Unit {
	result := []gomel.Unit{}
	candidates.Iterate(func(units []gomel.Unit) bool {
		// a)
		if len(units) == 1 {
			possibleCandidate := units[0]
			// b)
			if possibleCandidate.Level() != level {
				return true
			}
			// c)
			for _, u := range parents {
				if possibleCandidate.Below(u) {
					return true
				}
			}
			result = append(result, possibleCandidate)
		}
		return true
	})
	result = filterMaximal(result)
	return result
}

// pickMoreParents chooses from candidates, in a random order, up to limit units that fulfill
// "expand primes" rule with respect to vs.
func pickMoreParents(candidates []gomel.Unit, vs *visionSplit, enough func([]gomel.Unit) bool) []gomel.Unit {
	result := []gomel.Unit{}
	perm := rand.New(rand.NewSource(time.Now().Unix())).Perm(len(candidates))
	for _, i := range perm {
		if enough(result) {
			return result
		}
		c := candidates[i]
		if vs.newSeer(c) {
			result = append(result, c)
		}
	}
	return result
}

// combineParents merges two slices of units. Expects units in parents to be sorted via ascending level
// and units in newParents to all have the same level. Returned slice is also sorted and follows
// the same order as parents and newParents.
func combineParents(parents, newParents []gomel.Unit) []gomel.Unit {
	if len(newParents) == 0 {
		return parents
	}
	level := newParents[0].Level()
	result := []gomel.Unit{}
	for _, p := range parents {
		if p.Level() <= level {
			result = append(result, p)
		}
	}
	result = append(result, newParents...)
	for _, p := range parents {
		if p.Level() > level {
			result = append(result, p)
		}
	}
	return result
}

// hashes returns a slice with hashes of given units.
func hashes(units []gomel.Unit) []*gomel.Hash {
	result := make([]*gomel.Hash, len(units))
	for i, u := range units {
		result[i] = u.Hash()
	}
	return result
}

// NewNonSkippingUnit creates a preunit pu satisfying the following rules
// (1) level(pu) = level(predecessor(pu)) + 1
// (2) all the parents have level <= level(predecessor(pu))
// If such a unit cannot be created it returns an error.
//
// The procedure assumes no forks
// and should be used only in the setup phase.
// It also returns the level of the created unit.
func NewNonSkippingUnit(dag gomel.Dag, creator uint16, data []byte, rs gomel.RandomSource) (gomel.Preunit, int, error) {
	mu := dag.MaximalUnitsPerProcess()
	predecessor := getPredecessor(mu, creator)
	if predecessor == nil {
		return newDealingUnit(creator, dag.NProc(), data, rs), 0, nil
	}
	level := predecessor.Level()
	parentsOnLevel := uint16(1)
	parents := []gomel.Unit{predecessor}
	for pid := uint16(0); pid < dag.NProc(); pid++ {
		if pid == creator {
			continue
		}
		if len(mu.Get(pid)) != 0 {
			u := mu.Get(pid)[0]
			if u.Level() < level {
				if !u.Below(predecessor) {
					parents = append(parents, u)
				}
			} else {
				v := dag.PrimeUnits(level).Get(pid)[0]
				parents = append(parents, v)
				parentsOnLevel++
			}
		}
	}
	if dag.IsQuorum(parentsOnLevel) {
		// parents should be sorted by increasing level
		sort.Slice(parents[1:], func(i, j int) bool {
			return parents[i+1].Level() < parents[j+1].Level()
		})
		rsData, err := rs.DataToInclude(creator, parents, level+1)
		if err != nil {
			return nil, 0, err
		}
		return NewPreunit(creator, hashes(parents), data, rsData), level + 1, nil
	}
	return nil, 0, &noAvailableParents{}
}

// NewUnit creates a preunit for a given process aiming at desiredParents parents.
// The parents are chosen to satisfy the expand primes rule.
// If there don't exist at least two legal parents (one of which is the predecessor) it returns an error.
// It also returns an error if requirePrime is true and no prime can be produced.
// It returns the level of the created unit and whether it is prime.
func NewUnit(dag gomel.Dag, creator, desiredParents uint16, data []byte, rs gomel.RandomSource, requirePrime bool) (gomel.Preunit, int, bool, error) {
	mu := dag.MaximalUnitsPerProcess()
	predecessor := getPredecessor(mu, creator)
	// This is the first unit creator is creating, so it should be a dealing unit.
	if predecessor == nil {
		return newDealingUnit(creator, dag.NProc(), data, rs), 0, true, nil
	}
	predecessorLevel := predecessor.Level()
	parents := []gomel.Unit{predecessor}
	dagLevel := maxLevel(mu)
	resultLevel := predecessorLevel
	isPrime := false
	// We try picking units of the highest possible level as parents, going down if we haven't filled all the parent slots.
	// Usually this loop spans over at most two levels.
	for level := dagLevel; level >= predecessorLevel && (uint16(len(parents)) < desiredParents || (requirePrime && !isPrime)); level-- {
		candidates := getCandidatesAtLevel(mu, parents, level)
		vs := splitByBelow(dag.PrimeUnits(level), parents)
		newParents := pickMoreParents(candidates, vs, func(np []gomel.Unit) bool {
			if vs.hasQuorumIn(dag) {
				isPrime = true
			}
			totalLen := len(parents) + len(np)
			return (!requirePrime || isPrime) && uint16(totalLen) >= desiredParents
		})
		parents = combineParents(parents, newParents)
		if resultLevel == predecessorLevel && len(parents) > 1 {
			resultLevel = level
			isPrime = resultLevel > predecessorLevel
		}
		if vs.hasQuorumIn(dag) {
			isPrime = true
			if level == resultLevel {
				resultLevel++
			}
		}
	}
	if len(parents) < 2 || (requirePrime && !isPrime) {
		return nil, 0, false, &noAvailableParents{}
	}
	rsData, err := rs.DataToInclude(creator, parents, resultLevel)
	if err != nil {
		return nil, 0, false, err
	}
	return NewPreunit(creator, hashes(parents), data, rsData), resultLevel, isPrime, nil
}
