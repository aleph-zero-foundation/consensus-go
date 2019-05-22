package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

func checkExpandPrimes(p *Poset, pu gomel.Preunit) bool {
	parents := p.Get(pu.Parents())
	lastLevel := 0
	primesSeen := make(map[gomel.Hash]bool)
	for i, u := range parents {
		if i == 0 {
			lastLevel := u.Level()
			for _, prime := range p.getPrimeUnitsOnLevel(lastLevel) {
				if prime.Below(u) {
					primesSeen[*prime.Hash()] = true
				}
			}
		} else {
			if u.Level() < lastLevel {
				return false
			} else if u.Level() == lastLevel {
				ok := false
				for _, prime := range p.getPrimeUnitsOnLevel(lastLevel) {
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
				for _, prime := range p.getPrimeUnitsOnLevel(lastLevel) {
					if prime.Below(u) {
						primesSeen[*prime.Hash()] = true
					}
				}
			}
		}
	}
	return true
}
