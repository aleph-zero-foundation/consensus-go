package growing

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Assumes that prepare_unit(U) has been already called.
// Checks if the unit U is correct and follows the rules of creating units, i.e.:
// 1. Parents are created by pairwise different processes.
// 2. Satisfies forker-muting policy.
// 3. Satisfies the expand primes rule.
// 4. The random source data is OK.
func (dag *Dag) checkCompliance(u gomel.Unit, rs gomel.RandomSource) error {
	// 1. Parents are created by pairwise different processes.
	if err := checkParentsDiversity(u); err != nil {
		return err
	}

	// 2. Satisfies forker-muting policy
	if err := CheckForkerMuting(u.Parents()); err != nil {
		return err
	}

	// 3. Satisfies the expand primes rule
	if err := CheckExpandPrimes(dag, u.Parents()); err != nil {
		return err
	}

	// 4. The random source data is OK.
	if err := rs.CheckCompliance(u); err != nil {
		return err
	}
	return nil
}

func checkBasicParentsCorrectness(u gomel.Unit) error {
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
	if err := checkNoSelfForkingEvidence(u); err != nil {
		return err
	}

	return nil
}

// Check if all parents are created by pairwise different processes.
func checkParentsDiversity(u gomel.Unit) error {
	processFilter := map[uint16]bool{}
	for _, parent := range u.Parents() {
		if processFilter[parent.Creator()] {
			return gomel.NewComplianceError("Some of a unit's parents are created by the same process")
		}
		processFilter[parent.Creator()] = true
	}
	return nil
}

// Checks if the unit U does not provide evidence of its creator forking.
func checkNoSelfForkingEvidence(u gomel.Unit) error {
	if gomel.HasSelfForkingEvidence(u.Parents()) {
		return gomel.NewComplianceError("A unit is evidence of self forking")
	}
	return nil
}

// CheckForkerMuting checks if the set of units respects the forker-muting policy, i.e.:
// The following situation is not allowed:
//   - There exists a process j, s.t. one of the parents was created by j
//   AND
//   - one of the parents has evidence that j is forking.
func CheckForkerMuting(parents []gomel.Unit) error {
	for _, parent1 := range parents {
		for _, parent2 := range parents {
			if parent1 == parent2 {
				continue
			}
			if gomel.HasForkingEvidence(parent1, parent2.Creator()) {
				return gomel.NewComplianceError("Some parent has evidence of another parent being a forker")
			}
		}
	}
	return nil
}

// CheckExpandPrimes checks if the unit U respects the "expand primes" rule. Parents are checked consecutively. The first is
// just accepted. Then let L be the level of the last checked parent and P the set of creators of prime units of level L below
// all the parents checked up to now. The next parent must either have prime units of level L below it that are created by
// processes not in P, or have level greater than L.
func CheckExpandPrimes(dag gomel.Dag, parents []gomel.Unit) error {
	if len(parents) == 0 {
		return nil
	}

	wholeSet := make([]uint16, dag.NProc())
	for pid := range wholeSet {
		wholeSet[pid] = uint16(pid)
	}
	notSeenPrimes := wholeSet
	left := notSeenPrimes[:0]

	predecessor := parents[0]
	// predecessor can't have higher level than all other parents
	if predecessor.Level() > parents[len(parents)-1].Level() {
		return gomel.NewComplianceError("Expand primes rule violated - predecessor has higher level than any other parent")
	}

	level := parents[1].Level()
	for _, parent := range parents[1:] {
		if currentLevel := parent.Level(); currentLevel < level {
			return gomel.NewComplianceError("Expand primes rule violated - parents are not sorted in ascending order of levels")
		} else if currentLevel > level {
			level = currentLevel
			notSeenPrimes = wholeSet
			left = notSeenPrimes[:0]
		}

		isSubset := true
		parentsFloor := parent.Floor()
		for ix, pid := range notSeenPrimes {
			found := false
			for _, unit := range parentsFloor[pid] {
				if unit.Level() == level && !unit.Below(predecessor) {
					found = true
					isSubset = false
					break
				}
			}
			if !found {
				notSeenPrimes[ix] = notSeenPrimes[len(left)]
				left = append(left, pid)
			}
		}
		if isSubset {
			return gomel.NewComplianceError("Expand primes rule violated")
		}
		notSeenPrimes, left = left, notSeenPrimes[:0]
	}
	return nil
}
