package random

import (
	"sort"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"golang.org/x/crypto/sha3"
)

func CRP(rs gomel.RandomSource, p gomel.Poset, level int) []int {
	nProc := p.NProc()
	permutation := make([]int, nProc)
	priority := make([][]byte, nProc)
	for i := 0; i < nProc; i++ {
		permutation[i] = i
	}

	units := UnitsOnLevel(p, level)
	for _, u := range units {
		priority[u.Creator()] = make([]byte, 32)
		rBytes := rs.RandomBytes(u, level+3)
		if rBytes == nil {
			return nil
		}
		rBytes = append(rBytes, u.Hash()[:]...)
		sha3.ShakeSum128(priority[u.Creator()], rBytes)
	}

	sort.Slice(permutation, func(i, j int) bool {
		if priority[permutation[j]] == nil {
			return true
		}
		if priority[permutation[i]] == nil {
			return false
		}
		for x := 0; x < 32; x++ {
			if priority[permutation[i]][x] < priority[permutation[j]][x] {
				return true
			}
			if priority[permutation[i]][x] > priority[permutation[j]][x] {
				return false
			}
		}
		return (permutation[i] < permutation[j])
	})

	return permutation
}

func UnitsOnLevel(p gomel.Poset, level int) []gomel.Unit {
	result := []gomel.Unit{}
	su := p.PrimeUnits(level)
	if su != nil {
		su.Iterate(func(units []gomel.Unit) bool {
			if len(units) != 0 {
				result = append(result, units[0])
			}
			return true
		})
	}
	return result
}
