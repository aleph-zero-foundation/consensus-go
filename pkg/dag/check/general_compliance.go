package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// ExpandPrimes checks if the unit U respects the "expand primes" rule.
func ExpandPrimes(dag gomel.Dag) gomel.Dag {
	return Units(dag, func(u gomel.Unit) error { return ExpandPrimesCheck(dag, u.Parents()) })
}

// ExpandPrimesCheck checks if the unit U respects the "expand primes" rule. Parents are checked consecutively. The first is
// just accepted. Then let L be the level of the last checked parent and P the set of creators of prime units of level L below
// all the parents checked up to now. The next parent must either have prime units of level L below it that are created by
// processes not in P, or have level greater than L.
func ExpandPrimesCheck(dag gomel.Dag, parents []gomel.Unit) error {
	if len(parents) == 0 {
		return nil
	}

	wholeSet := make([]int, dag.NProc())
	for pid := 0; pid < len(wholeSet); pid++ {
		wholeSet[pid] = pid
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
