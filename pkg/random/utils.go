package random

import (
	"encoding/binary"
	"sort"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

// CRP returns a common random permutation based on a RandomBytes method
// from given random source, as explained in the whitepaper.
func CRP(rs gomel.RandomSource, dag gomel.Dag, level int) []int {
	nProc := dag.NProc()
	permutation := make([]int, nProc)
	priority := make([][]byte, nProc)
	for i := 0; i < nProc; i++ {
		permutation[i] = i
	}

	units := UnitsOnLevel(dag, level)
	if len(units) == 0 {
		return nil
	}

	for _, u := range units {
		priority[u.Creator()] = make([]byte, 32)

		rBytes := rs.RandomBytes(u, level+3)
		if rBytes == nil {
			return nil
		}

		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(u.Creator()))
		rBytes = append(rBytes, buf...)
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
		panic("hash collision")
		return (permutation[i] < permutation[j])
	})

	return permutation
}

// UnitsOnLevel returns all the prime units in dag on a given level
func UnitsOnLevel(dag gomel.Dag, level int) []gomel.Unit {
	result := []gomel.Unit{}
	su := dag.PrimeUnits(level)
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
