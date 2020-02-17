package tests

import (
	"strconv"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type testRandomSource struct {
}

// NewTestRandomSource returns a simple RandomSource for testing.
func NewTestRandomSource(dag gomel.Dag) gomel.RandomSource {
	return &testRandomSource{}
}

// RandomBytes returns a sequence of "random" bits for a given unit.
// It bases the sequence only on the pid and level, ignoring the unit itself.
func (rs *testRandomSource) RandomBytes(pid uint16, level int) []byte {
	answer := make([]byte, 32)
	answer = append(answer, []byte(strconv.Itoa(int(pid)+level))...)
	return answer
}

// ToInclude always returns nil.
func (*testRandomSource) DataToInclude([]gomel.Unit, int) ([]byte, error) {
	return nil, nil
}
