package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// BasicCompliance returns a version of the dag that will check the following notion of correctness:
//  1. If a unit has nProc parents such that the i-th parent is created by the i-th process.
//  2. A unit has to have a predecessor or have all parents nil.
//  3. A unit is a prime unit.
func BasicCompliance(dag gomel.Dag) {
	dag.AddCheck(func(u gomel.Unit) error { return checkBasicCorrectness(u, dag.NProc()) })
}

func checkBasicCorrectness(u gomel.Unit, nProc uint16) error {
	if len(u.Parents()) != int(nProc) {
		return gomel.NewComplianceError("Wrong number of parents")
	}
	nonNilParents := uint16(0)
	for i := uint16(0); i < nProc; i++ {
		if u.Parents()[i] == nil {
			continue
		}
		nonNilParents++
		if u.Parents()[i].Creator() != i {
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
