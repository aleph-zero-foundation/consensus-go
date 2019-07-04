package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type testRandomSource struct {
	nProc int
}

// NewTestRandomSource returns a simple RandomSource for testing
func NewTestRandomSource(poset gomel.Poset) gomel.RandomSource {
	return &testRandomSource{
		nProc: poset.NProc(),
	}
}

// GetCRP is a dummy implementation of a common random permutation
func (rs *testRandomSource) GetCRP(nonce int) []int {
	permutation := make([]int, rs.nProc)
	for i := 0; i < rs.nProc; i++ {
		permutation[i] = (i + nonce) % rs.nProc
	}
	return permutation
}

// RandomBytes returns a sequence of random bits for a given unit and nonce
// it returns hash of u
func (rs *testRandomSource) RandomBytes(uTossing gomel.Unit, nonce int) []byte {
	return uTossing.Hash()[:]
}

// Update updates the RandomSource with data included in the preunit
func (*testRandomSource) Update(gomel.Preunit) error {
	return nil
}

// Rollback rolls back an update
func (*testRandomSource) Rollback(gomel.Preunit) {
}

// ToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (*testRandomSource) DataToInclude(int, []gomel.Unit, int) []byte {
	return nil
}
