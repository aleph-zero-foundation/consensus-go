package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// BasicCorrectness returns a version of the dag that will check the following notion of correctness:
//  1. If a unit has nProc parents such that the i-th parent is created by the i-th process.
//  2. A unit has to have a predecessor or have all parents nil.
//  3. A unit is a prime unit.
func BasicCorrectness(u gomel.Unit, dag gomel.Dag) error {
	parents := u.Parents()
	nProc := dag.NProc()
	if len(parents) != int(nProc) {
		return gomel.NewComplianceError("Wrong number of parents")
	}
	nonNilParents := uint16(0)
	for i := uint16(0); i < nProc; i++ {
		if parents[i] == nil {
			continue
		}
		nonNilParents++
		if parents[i].Creator() != i {
			return gomel.NewComplianceError("i-th parent not created by i-th process")
		}
	}
	if gomel.Predecessor(u) == nil && nonNilParents > 0 {
		return gomel.NewComplianceError("unit without a predecessor but with other parents")
	}
	if gomel.Predecessor(u) != nil && gomel.Predecessor(u).Level() >= u.Level() {
		return gomel.NewComplianceError("non-prime unit")
	}
	return nil
}
