package creating

import (
	"math/rand"
	"sort"
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
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
func newDealingUnit(creator, NProc int, data []byte) gomel.Preunit {
	tc := tcoin.Deal(NProc, NProc/3+1)
	return NewPreunit(creator, []*gomel.Hash{}, data, nil, tc)
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

// filterNotBelow picks all the units in su that are not below any of the units.
func filterNotBelow(su gomel.SlottedUnits, units []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	su.Iterate(func(primes []gomel.Unit) bool {
		for _, prime := range primes {
			if !gomel.BelowAny(prime, units) {
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
		if gomel.AboveAny(c, nvp) {
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
func hashes(units []gomel.Unit) []*gomel.Hash {
	result := make([]*gomel.Hash, len(units))
	for i, u := range units {
		result[i] = u.Hash()
	}
	return result
}

// levelFromParents calculates level of the unit under construction that will have given set parents
// Expects units in parents to be sorted via ascending level
// It uses dfs on the maximal level of parents
func levelFromParents(parents []gomel.Unit, poset gomel.Poset) int {
	if len(parents) == 0 {
		return 0
	}

	level := parents[len(parents)-1].Level()
	procSeen := make(map[int]bool)
	unitsSeen := make(map[gomel.Hash]bool)
	stack := []gomel.Unit{}
	for _, u := range parents {
		if u.Level() != level {
			continue
		}
		stack = append(stack, u)
		unitsSeen[*u.Hash()] = true
		procSeen[u.Creator()] = true
		if poset.IsQuorum(len(procSeen)) {
			return level + 1
		}
	}
	for len(stack) > 0 {
		w := stack[len(stack)-1]
		stack = stack[:(len(stack) - 1)]

		for _, v := range w.Parents() {
			if v.Level() == level && !unitsSeen[*v.Hash()] {
				stack = append(stack, v)
				unitsSeen[*v.Hash()] = true
				procSeen[v.Creator()] = true
				if poset.IsQuorum(len(procSeen)) {
					return level + 1
				}
			}
		}
	}
	return level
}

// firstDealingUnitFromParents takes parents of the unit under construction
// and calculates the first (sorted with respect to CRP on level of the unit) dealing unit
// that is below the unit under construction
func firstDealingUnitFromParents(parents []gomel.Unit, level int, poset gomel.Poset) gomel.Unit {
	dealingUnits := poset.PrimeUnits(0)
	for _, dealer := range poset.GetCRP(level) {
		// We are only checking if there are forked dealing units created by the dealer
		// below the unit under construction.
		// We could check if we have evidence that the dealer is forking
		// but this is expensive without access to floors.
		var result gomel.Unit
		var dealersDealingUnits = dealingUnits.Get(dealer)
		sort.Slice(dealersDealingUnits, func(i, j int) bool {
			return dealersDealingUnits[i].Hash().LessThan(dealersDealingUnits[j].Hash())
		})
		for _, u := range dealersDealingUnits {
			if gomel.BelowAny(u, parents) {
				if result != nil {
					// we see forked dealing unit
					result = nil
					break
				} else {
					result = u
				}
			}
		}
		if result != nil {
			return result
		}
	}
	return nil
}

// createCoinShare takes parents of the unit under construction
// if the unit will be a prime unit it returns coin share to include in the unit
// otherwise it returns nil
func createCoinShare(parents []gomel.Unit, poset gomel.Poset) *tcoin.CoinShare {
	level := levelFromParents(parents, poset)
	if level == parents[0].Level() {
		return nil
	}
	fdu := firstDealingUnitFromParents(parents, level, poset)
	tc := poset.ThresholdCoin(fdu.Hash())
	if tc == nil {
		return nil
	}
	return tc.CreateCoinShare(level)
}

// NewUnit creates a preunit for a given process with at most maximumParents parents.
// The parents are chosen to satisfy the expand primes rule.
// If there don't exist at least two legal parents (one of which is the predecessor) it returns an error.
func NewUnit(poset gomel.Poset, creator int, maximumParents int, data []byte) (gomel.Preunit, error) {
	mu := poset.MaximalUnitsPerProcess()
	predecessor := getPredecessor(mu, creator)
	// This is the first unit creator is creating, so it should be a dealing unit.
	if predecessor == nil {
		return newDealingUnit(creator, poset.NProc(), data), nil
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
	cs := createCoinShare(parents, poset)
	return NewPreunit(creator, hashes(parents), data, cs, nil), nil
}
