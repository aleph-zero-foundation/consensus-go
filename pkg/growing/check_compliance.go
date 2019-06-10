package growing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Assumes that prepare_unit(U) has been already called.
// Checks if the unit U is correct and follows the rules of creating units, i.e.:
// 1. Parents are created by pairwise different processes.
// 2. U does not provide evidence of its creator forking
// 3. Satisfies forker-muting policy.
// 4. Satisfies the expand primes rule.
// 5. The coinshares are OK, i.e., U contains exactly the coinshares it is supposed to contain.
func (p *Poset) checkCompliance(u gomel.Unit) error {
	if gomel.Dealing(u) {
		// This is a dealing unit, and its signature is correct --> we only need to check whether threshold coin is included
		return checkThresholdCoinIncluded(u)
	}
	// 1. Parents are created by pairwise different processes.
	if err := checkParentsDiversity(u); err != nil {
		return err
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

func (p *Poset) checkBasicParentsCorrectness(u gomel.Unit) error {
	if len(u.Parents()) == 0 && gomel.Dealing(u) {
		return nil
	}
	if len(u.Parents()) < 2 {
		return gomel.NewComplianceError("Not enough parents")
	}
	selfPredecessor, err := gomel.Predecessor(u)
	if err != nil {
		return gomel.NewComplianceError("Can not retrieve unit's self-predecessor")
	}
	firstParent := u.Parents()[0]
	if firstParent.Creator() != u.Creator() {
		return gomel.NewComplianceError("Not descendant of first parent")
	}
	// self-predecessor and the first unit on the Parents list should be equal
	if firstParent != selfPredecessor {
		return gomel.NewComplianceError("First parent of a unit is not equal to its self-predecessor")
	}

	return nil
}

// Check if all parents are created by pairwise different processes.
func checkParentsDiversity(u gomel.Unit) error {

	processFilter := map[int]bool{}
	for _, parent := range u.Parents() {
		if processFilter[parent.Creator()] {
			return gomel.NewComplianceError("Some of a unit's parents are created by the same process")
		}
		processFilter[parent.Creator()] = true
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
		return gomel.NewComplianceError("A unit is evidence of self forking")
	}
	return nil
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
// Then let L be the level of the last checked parent and P the set of creators of prime units of level L below all the parents
// checked up to now. The next parent must must either have prime units of level L below it that are created by processes
//  not in P, or have level greater than L.
func (p *Poset) checkExpandPrimes(u gomel.Unit) error {
	seenPrimes := make([]bool, p.NProc())
	level := u.Parents()[0].Level()
	for _, parent := range u.Parents() {
		if currentLevel := parent.Level(); currentLevel > level {
			level = currentLevel
			for pid := range seenPrimes {
				seenPrimes[pid] = false
			}
		}

		isSubset := true
		parentsFloor := parent.Floor()
		for pid, seen := range seenPrimes {
			if seen {
				continue
			}
			for _, unit := range parentsFloor[pid] {
				if unit.Level() == level {
					seenPrimes[pid] = true
					isSubset = false
					break
				}
			}
		}
		if isSubset {
			return gomel.NewComplianceError("Expand primes rule violated")
		}
	}
	return nil
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
