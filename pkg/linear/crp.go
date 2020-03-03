package linear

import (
	"sort"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

type commonRandomPermutation struct {
	dag            gomel.Dag
	randomSource   gomel.RandomSource
	crpFixedPrefix uint16
}

func newCommonRandomPermutation(dag gomel.Dag, randomSource gomel.RandomSource, crpFixedPrefix uint16) *commonRandomPermutation {
	return &commonRandomPermutation{dag: dag, randomSource: randomSource, crpFixedPrefix: crpFixedPrefix}
}

// CRPIterate iterates over all the prime units on a given level in random order.
// It calls the given work function on each of the units until the function
// returns false or the contents run out.
// The underlying random permutation of units is generated in two steps
// (1) the prefix is based only on the previous timing unit and hashes of units
// (2) the suffix is computed using the random source
// The second part of the permutation is being calculated only when needed,
// i.e. the given work function returns true on all the units in the prefix.
//
// The function itself returns
// - false when generating the suffix of the permutation failed (because the dag
//   hasn't reached a level high enough to reveal the randomBytes needed)
// - true otherwise
func (crp *commonRandomPermutation) CRPIterate(level int, previousTU gomel.Unit, work func(gomel.Unit) bool) bool {
	prefix, sufix := splitProcesses(crp.dag.NProc(), crp.crpFixedPrefix, level, previousTU)

	perm := defaultPermutation(crp.dag, level, prefix)
	for _, u := range perm {
		if !work(u) {
			return true
		}
	}

	perm, ok := randomPermutation(crp.randomSource, crp.dag, level, sufix)
	if !ok {
		return false
	}
	for _, u := range perm {
		if !work(u) {
			return true
		}
	}
	return true
}

func splitProcesses(nProc, prefixLen uint16, level int, tu gomel.Unit) ([]uint16, []uint16) {
	if prefixLen > nProc {
		prefixLen = nProc
	}
	pids := make([]uint16, nProc)
	for pid := range pids {
		pids[pid] = uint16((pid + level) % int(nProc))
	}
	if tu == nil {
		return pids[:prefixLen], pids[prefixLen:]
	}
	for pid := range pids {
		pids[pid] = (pids[pid] + tu.Creator()) % nProc
	}
	return pids[:prefixLen], pids[prefixLen:]
}

func defaultPermutation(dag gomel.Dag, level int, pids []uint16) []gomel.Unit {
	permutation := []gomel.Unit{}

	for _, pid := range pids {
		permutation = append(permutation, dag.UnitsOnLevel(level).Get(pid)...)
	}

	sort.Slice(permutation, func(i, j int) bool {
		return permutation[i].Hash().LessThan(permutation[j].Hash())
	})
	return permutation
}

func randomPermutation(rs gomel.RandomSource, dag gomel.Dag, level int, pids []uint16) ([]gomel.Unit, bool) {
	permutation := []gomel.Unit{}
	priority := make(map[gomel.Unit][]byte)

	for _, pid := range pids {
		randomBytes := rs.RandomBytes(pid, level+5)
		if randomBytes == nil {
			return nil, false
		}
		rbLen := len(randomBytes)
		units := dag.UnitsOnLevel(level).Get(pid)
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
