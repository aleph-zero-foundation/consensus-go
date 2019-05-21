package creating

import (
	"math/rand"
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type noAvailableParents struct{}

func (e *noAvailableParents) Error() string {
	return "No legal parents for the unit."
}

// getPredecessor picks one of the units in mu produced by the given creator.
func getPredecessor(mu gomel.SlottedUnits, creator int) gomel.Unit {
	maxUnits := mu.Get(creator)
	if len(maxUnits) == 0 {
		return nil
	}
	return maxUnits[0]
}

// newDealingUnit creates a new preunit with the given creator and no parents.
func newDealingUnit(creator int, data []byte) gomel.Preunit {
	return NewPreunit(creator, []gomel.Hash{}, data)
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

// belowAny checks if a given unit is below any of the units.
func belowAny(unit gomel.Unit, units []gomel.Unit) bool {
	for _, u := range units {
		if unit.Below(u) {
			return true
		}
	}
	return false
}

// aboveAny checks if a given unit is above any of the units.
func aboveAny(unit gomel.Unit, units []gomel.Unit) bool {
	for _, u := range units {
		if u.Below(unit) {
			return true
		}
	}
	return false
}

// filterNotBelow picks all the units in su that are not below any of the units.
func filterNotBelow(su gomel.SlottedUnits, units []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	su.Iterate(func(primes []gomel.Unit) bool {
		for _, prime := range primes {
			if !belowAny(prime, units) {
				result = append(result, prime)
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

// filterOutBelow chooses from units the ones that are not below the given unit.
func filterOutBelow(units []gomel.Unit, unit gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	for _, u := range units {
		if !u.Below(unit) {
			result = append(result, u)
		}
	}
	return result
}

// pickMoreParents chooses from candidates, in a random order, up to limit units that fulfill
// "expand primes" rule with respect to prime units contained in nvp.
func pickMoreParents(candidates, nvp []gomel.Unit, limit int) []gomel.Unit {
	result := []gomel.Unit{}
	perm := rand.New(rand.NewSource(time.Now().Unix())).Perm(len(candidates))
	for _, i := range perm {
		if len(result) == limit {
			return result
		}
		c := candidates[i]
		if aboveAny(c, nvp) {
			result = append(result, c)
			nvp = filterOutBelow(nvp, c)
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
func hashes(units []gomel.Unit) []gomel.Hash {
	result := make([]gomel.Hash, len(units))
	for i, u := range units {
		result[i] = *u.Hash()
	}
	return result
}

// NewUnit creates a preunit for a given process with at most maximumParents parents.
// The parents are chosen to satisfy the expand primes rule.
// If there don't exist at least two legal parents (one of which is the predecessor) it returns an error.
func NewUnit(poset gomel.Poset, creator int, maximumParents int, data []byte) (gomel.Preunit, error) {
	mu := poset.MaximalUnitsPerProcess()
	predecessor := getPredecessor(mu, creator)
	// This is the first unit creator is creating, so it should be a dealing unit.
	if predecessor == nil {
		return newDealingUnit(creator, data), nil
	}
	parents := []gomel.Unit{predecessor}
	posetLevel := maxLevel(mu)
	// We try picking units of the highest possible level as parents, going down if we haven't filled all the parent slots.
	// Usually this loop spans over at most two levels.
	for level := posetLevel; level >= predecessor.Level() && len(parents) < maximumParents; level-- {
		candidates := getCandidatesAtLevel(mu, parents, level)
		nvp := filterNotBelow(poset.PrimeUnits(level), parents)
		parents = combineParents(parents, pickMoreParents(candidates, nvp, maximumParents-len(parents)))
	}
	if len(parents) < 2 {
		return nil, &noAvailableParents{}
	}
	return NewPreunit(creator, hashes(parents), data), nil
}
