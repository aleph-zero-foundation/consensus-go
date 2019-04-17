package growing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Assumes that prepare_unit(U) has been already called.
// Checks if the unit U is correct and follows the rules of creating units, i.e.:
// 1. Parents of U are correct (exist in the poset, etc.)
// 2. U does not provide evidence of its creator forking
// 3. Satisfies forker-muting policy.
// 4. Satisfies the expand primes rule.
// 5. The coinshares are OK, i.e., U contains exactly the coinshares it is supposed to contain.
func (p *Poset) checkCompliance(u gomel.Unit) error {
	// 1. Parents of U are correct
	if err := checkParentCorrectness(u); err != nil {
		return err
	}

	if gomel.Dealing(u) {
		// This is a dealing unit, and its signature is correct --> we only need to check whether threshold coin is included
		if err := checkThresholdCoinIncluded(u); err != nil {
			return err
		} else {
			return nil
		}
	}

	// 2. U does not provide evidence of its creator forking
	if err := checkNoSelfForkingEvidence(u); err != nil {
		return err
	}

	// 3. Satisfies forker-muting policy
	if err := checkForkerMuting(u); err != nil {
		return err
	}

	// 4. Satisfies the expand primes rule
	if err := p.checkExpandPrimes(u); err != nil {
		return err
	}

	// 5. Coinshares are OK
	if err := p.verifyCoinShares(u); err != nil {
		return err
	}
	return nil
}

// Checks whether U has correct parents:
// 1. One of the parents, called by self-predecessor, was created by U's creator and has one less height than U.
// 2. If U has >=2 parents then all parents are created by pairwise different processes.
// This method assumes that parents of a given unit are already added to the poset.
func checkParentCorrectness(u gomel.Unit) error {
	// NOTE: this is also verified during unit's addition, currently in precheck

	// 1. The first parent was created by U's creator and has one less height than U.
	if selfPredecesor, err := gomel.Predecessor(u); err == nil {
		if selfPredecesor.Creator() != u.Creator() {
			return gomel.NewComplianceError("First parent was not created by the same process")
		}
		if selfPredecesor.Height()+1 != u.Height() {
			return gomel.NewComplianceError("Invalid value of the 'Height' property")
		}
	}

	// 2. If U has parents created by pairwise different processes.
	processFilter := map[int]bool{}
	if len(u.Parents()) >= 2 {
		for _, parent := range u.Parents() {
			if processFilter[parent.Creator()] {
				return gomel.NewComplianceError("Some of the unit's parents are created by the same process")
			} else {
				processFilter[parent.Creator()] = true
			}
		}
	}

	return nil
}

// Checks whether the dealing unit U has a threshold coin included.
func checkThresholdCoinIncluded(u gomel.Unit) error {
	// TODO: implement

	return nil
}

// Checks if the unit U does not provide evidence of its creator forking.
func checkNoSelfForkingEvidence(u gomel.Unit) error {
	if u.HasForkingEvidence(u.Creator()) {
		return gomel.NewComplianceError("A unit is an evidence of self forking")
	} else {
		return nil
	}
}

// Checks if the unit U respects the forker-muting policy, i.e.:
// The following situation is not allowed:
//   - There exists a process j, s.t. one of U's parents was created by j
//   AND
//   - U has as one of the parents a unit that has evidence that j is forking.
func checkForkerMuting(u gomel.Unit) error {
	for _, parent1 := range u.Parents() {
		for _, parent2 := range u.Parents() {
			if parent1 == parent2 {
				continue
			}
			if parent1.HasForkingEvidence(parent2.Creator()) {
				return gomel.NewComplianceError("Some parent has evidence of another parent being a forker")
			}
		}
	}
	return nil
}

// Checks if the unit U respects the "expand primes" rule. Parents are checked consecutively. The first is just accepted.
// Then let L be the level of the last checked parent and P the set of prime units of level L below all the parents checked up
// to now. The next parent must must either have prime units of level L below it that are not in P, or have level greater than L.
func (p *Poset) checkExpandPrimes(u gomel.Unit) error {
	// Special case of dealing units
	if gomel.Dealing(u) {
		return nil
	}

	selfPredecessor, err := gomel.Predecessor(u)
	if err != nil {
		return gomel.NewComplianceError("can not retrieve unit's self predecessor")
	}
	level := selfPredecessor.Level()
	primeBelowParents := map[gomel.Unit]bool{}
	for _, prime := range p.getPrimeUnitsAtLevelBelowUnit(level, u.Parents()[0]) {
		primeBelowParents[prime] = true
	}
	for _, parent := range u.Parents()[1:] {
		if parent.Level() > level {
			level = parent.Level()
			primeBelowParents = map[gomel.Unit]bool{}
		}
		primeBelowV := p.getPrimeUnitsAtLevelBelowUnit(level, parent)
		// If Parent has only a subset of previously seen prime units below it we have a violation
		isSubset := true
		for _, prime := range primeBelowV {
			if !primeBelowParents[prime] {
				isSubset = false
			}
			primeBelowParents[prime] = true
		}
		if isSubset {
			return gomel.NewComplianceError("Expand primes rule violated")
		}
	}
	return nil
}

func (p *Poset) getPrimeUnitsAtLevelBelowUnit(level int, u gomel.Unit) []gomel.Unit {
	var result []gomel.Unit
	primes := p.PrimeUnits(level)
	for process := 0; process < p.nProcesses; process++ {
		for _, prime := range primes.Get(process) {
			if prime.Below(u) {
				result = append(result, prime)
			}
		}
	}
	return result
}

func (p *Poset) verifyCoinShares(u gomel.Unit) error {
	if !gomel.Prime(u) || gomel.Dealing(u) {
		return nil
	}
	return p.checkCoinShares(u)
}

// Checks coin shares of a prime unit that is not a dealing unit.
func (p *Poset) checkCoinShares(u gomel.Unit) error {
	// TODO: implement

	return nil
}
