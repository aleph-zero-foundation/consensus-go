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

func getPredecessor(mu gomel.SlottedUnits, creator int) gomel.Unit {
	maxUnits := mu.Get(creator)
	if len(maxUnits) == 0 {
		return nil
	}
	return maxUnits[0]
}

func newDealingUnit(creator int) gomel.Preunit {
	return newPreunit(creator, []gomel.Hash{})
}

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

func belowAny(unit gomel.Unit, units []gomel.Unit) bool {
	for _, u := range units {
		if unit.Below(u) {
			return true
		}
	}
	return false
}

func getNonVisiblePrimes(pu gomel.SlottedUnits, units []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	pu.Iterate(func(primes []gomel.Unit) bool {
		for _, prime := range primes {
			if !belowAny(prime, units) {
				result = append(result, prime)
			}
		}
		return true
	})
	return result
}

func maximalIn(u gomel.Unit, units []gomel.Unit) bool {
	for _, v := range units {
		if u != v && u.Below(v) {
			return false
		}
	}
	return true
}

func filterMaximal(units []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	for _, u := range units {
		if maximalIn(u, units) {
			result = append(result, u)
		}
	}
	return result
}

func getCandidatesAtLevel(candidates gomel.SlottedUnits, parents []gomel.Unit, level int) []gomel.Unit {
	result := []gomel.Unit{}
	candidates.Iterate(func(units []gomel.Unit) bool {
		// Only pick candidates from nonforking processes.
		if len(units) == 1 {
			possibleCandidate := units[0]
			if possibleCandidate.Level() != level {
				return true
			}
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

func filterOutBelow(unit gomel.Unit, units []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	for _, u := range units {
		if !u.Below(unit) {
			result = append(result, u)
		}
	}
	return result
}

func checkCandidate(c gomel.Unit, nvp []gomel.Unit) bool {
	for _, p := range nvp {
		if p.Below(c) {
			return true
		}
	}
	return false
}

func pickMoreParents(nvp []gomel.Unit, candidates []gomel.Unit, limit int) []gomel.Unit {
	result := []gomel.Unit{}
	// Try the candidates in a random order.
	perm := rand.New(rand.NewSource(time.Now().Unix())).Perm(len(candidates))
	for _, i := range perm {
		if len(result) == limit {
			return result
		}
		c := candidates[i]
		if checkCandidate(c, nvp) {
			result = append(result, c)
			nvp = filterOutBelow(c, nvp)
		}
	}
	return result
}

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
func NewUnit(poset gomel.Poset, creator int, maximumParents int) (gomel.Preunit, error) {
	mu := poset.MaximalUnitsPerProcess()
	predecessor := getPredecessor(mu, creator)
	// This is the first unit creator is creating, so it should be a dealing unit.
	if predecessor == nil {
		return newDealingUnit(creator), nil
	}
	parents := []gomel.Unit{predecessor}
	posetLevel := maxLevel(mu)
	// We try picking units of the highest possible level as parents, going down if we haven't filled all the parent slots.
	// Usually this loop spans over at most two levels.
	for level := posetLevel; level >= predecessor.Level() && len(parents) < maximumParents; level-- {
		candidates := getCandidatesAtLevel(mu, parents, level)
		nvp := getNonVisiblePrimes(poset.PrimeUnits(level), parents)
		parents = combineParents(parents, pickMoreParents(nvp, candidates, maximumParents-len(parents)))
	}
	if len(parents) < 2 {
		return nil, &noAvailableParents{}
	}
	return newPreunit(creator, hashes(parents)), nil
}
