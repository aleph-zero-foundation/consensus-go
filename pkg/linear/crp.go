package linear

import (
	"sort"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

// crpIterate iterates over all the units on a given level in random order
// until the given function returns false.
// The underlying random permutation of units is generated in two steps
// (1) the prefix is based only on the previous timing unit and hashes of units
// (2) the sufix is computed using the random source
// Second part of the permutation is being calculated only when needed.
// i.e. given function returns true on all the units in the prefix.
//
// The function itself returns
// - false when generating sufix of the permutation failed (because the dag
//   haven't reached high enough level to reveal randomBytes needed)
// - true otherwise
func (o *ordering) crpIterate(level int, previousTU gomel.Unit, f func(gomel.Unit) bool) bool {
	prefix, sufix := splitProcesses(o.dag.NProc(), o.crpFixedPrefix, level, previousTU)

	perm := defaultPermutation(o.dag, level, prefix)
	for _, u := range perm {
		if !f(u) {
			return true
		}
	}

	perm, ok := randomPermutation(o.randomSource, o.dag, level, sufix)
	if !ok {
		return false
	}
	for _, u := range perm {
		if !f(u) {
			return true
		}
	}
	return true
}

func splitProcesses(nProc int, prefixLen int, level int, tu gomel.Unit) ([]int, []int) {
	if prefixLen > nProc {
		prefixLen = nProc
	}
	pids := make([]int, nProc)
	for pid := range pids {
		pids[pid] = (pid + level) % nProc
	}
	if tu == nil {
		return pids[:prefixLen], pids[prefixLen:]
	}
	// Here we can use previous timing unit to return a result
	// which cannot be predicted before previous timing was chosen
	return pids[:prefixLen], pids[prefixLen:]
}

func defaultPermutation(dag gomel.Dag, level int, pids []int) []gomel.Unit {
	permutation := []gomel.Unit{}

	for _, pid := range pids {
		permutation = append(permutation, dag.PrimeUnits(level).Get(pid)...)
	}

	sort.Slice(permutation, func(i, j int) bool {
		return permutation[i].Hash().LessThan(permutation[j].Hash())
	})
	return permutation
}

func randomPermutation(rs gomel.RandomSource, dag gomel.Dag, level int, pids []int) ([]gomel.Unit, bool) {
	permutation := []gomel.Unit{}
	priority := make(map[gomel.Unit][]byte)

	for _, pid := range pids {
		randomBytes := rs.RandomBytes(pid, level)
		if randomBytes == nil {
			return nil, false
		}
		rbLen := len(randomBytes)
		units := dag.PrimeUnits(level).Get(pid)
		for _, u := range units {
			randomBytes = append(randomBytes[:rbLen], (*u.Hash())[:]...)
			priority[u] = make([]byte, 32)
			sha3.ShakeSum128(priority[u], randomBytes)
		}
		permutation = append(permutation, units...)
	}

	sort.Slice(permutation, func(i, j int) bool {
		if priority[permutation[j]] == nil {
			return true
		}
		if priority[permutation[i]] == nil {
			return false
		}
		for x := range priority[permutation[i]] {
			if priority[permutation[i]][x] < priority[permutation[j]][x] {
				return true
			}
			if priority[permutation[i]][x] > priority[permutation[j]][x] {
				return false
			}
		}
		panic("two elements with equal priority")
	})
	return permutation, true
}
