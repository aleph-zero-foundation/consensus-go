package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

func checkExpandPrimes(dag *Dag, pu gomel.Preunit) bool {
	parents := dag.Get(pu.Parents())
	lastLevel := -1
	var primesSeen map[gomel.Hash]bool
	for _, u := range parents {
		if u.Level() < lastLevel {
			return false
		} else if u.Level() == lastLevel {
			ok := false
			for _, prime := range dag.getPrimeUnitsOnLevel(lastLevel) {
				if !primesSeen[*prime.Hash()] && prime.Below(u) {
					ok = true
					primesSeen[*prime.Hash()] = true
				}
			}
			if !ok {
				return false
			}
		} else {
			lastLevel = u.Level()
			primesSeen = make(map[gomel.Hash]bool)
			for _, prime := range dag.getPrimeUnitsOnLevel(lastLevel) {
				if prime.Below(u) {
					primesSeen[*prime.Hash()] = true
				}
			}
		}
	}
	return true
}
