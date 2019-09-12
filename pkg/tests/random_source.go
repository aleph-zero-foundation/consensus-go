package tests

import (
	"strconv"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type testRandomSource struct {
	nProc uint16
}

// NewTestRandomSource returns a simple RandomSource for testing.
func NewTestRandomSource() gomel.RandomSource {
	return &testRandomSource{}
}

// Init initializes the random source with the given dag.
func (rs *testRandomSource) Init(dag gomel.Dag) {
	rs.nProc = dag.NProc()
}

// RandomBytes returns a sequence of "random" bits for a given unit.
// It bases the sequence only on the pid and level, ignoring the unit itself.
func (rs *testRandomSource) RandomBytes(pid uint16, level int) []byte {
	answer := make([]byte, 32)
	answer = append(answer, []byte(strconv.Itoa(int(pid)+level))...)
	return answer
}

// Update is a noop.
func (*testRandomSource) Update(gomel.Unit) {
}

// CheckCompliance accepts everything.
func (rs *testRandomSource) CheckCompliance(gomel.Unit) error {
	return nil
}

// ToInclude always returns nil.
func (*testRandomSource) DataToInclude(uint16, []gomel.Unit, int) ([]byte, error) {
	return nil, nil
}
