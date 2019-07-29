package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type testRandomSource struct {
	nProc int
}

// NewTestRandomSource returns a simple RandomSource for testing
func NewTestRandomSource() gomel.RandomSource {
	return &testRandomSource{}
}

// Init initialize the random source with given dag
func (rs *testRandomSource) Init(dag gomel.Dag) {
	rs.nProc = dag.NProc()
}

// GetCRP is a dummy implementation of a common random permutation
func (rs *testRandomSource) GetCRP(nonce int) []int {
	permutation := make([]int, rs.nProc)
	for i := 0; i < rs.nProc; i++ {
		permutation[i] = (i + nonce) % rs.nProc
	}
	return permutation
}

// RandomBytes returns a sequence of random bits for a given unit
// it returns hash of u
func (rs *testRandomSource) RandomBytes(uTossing gomel.Unit, _ int) []byte {
	return uTossing.Hash()[:]
}

// Update updates the RandomSource with data included in the unit
func (*testRandomSource) Update(gomel.Unit) {
}

// CheckCompliance checks wheather the random source data incldued in the unit is correct
func (rs *testRandomSource) CheckCompliance(gomel.Unit) error {
	return nil
}

// ToInclude returns data which should be included in the unit under creation
// with given creator and set of parents.
func (*testRandomSource) DataToInclude(int, []gomel.Unit, int) []byte {
	return nil
}
