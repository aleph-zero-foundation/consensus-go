package tests

import (
	"strconv"

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

// RandomBytes returns a sequence of random bits for a given unit
// it returns hash of u
func (rs *testRandomSource) RandomBytes(pid, level int) []byte {
	answer := make([]byte, 32)
	answer = append(answer, []byte(strconv.Itoa(pid+level))...)
	return answer
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
