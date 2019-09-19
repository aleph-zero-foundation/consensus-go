package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// BasicCompliance returns a version of the dag that will check the following notion of correctness:
//  1. If a unit has 0 parents and is a dealing unit it is correct, otherwise
//  2. A unit has to have at least two parents.
//  3. A unit has to have a predecessor with the same creator.
func BasicCompliance(dag gomel.Dag) gomel.Dag {
	return Units(dag, checkBasicCorrectness)
}

func checkBasicCorrectness(u gomel.Unit) error {
	if len(u.Parents()) == 0 && gomel.Dealing(u) {
		return nil
	}
	if len(u.Parents()) < 2 {
		return gomel.NewComplianceError("Not enough parents")
	}
	selfPredecessor, err := gomel.Predecessor(u)
	if err != nil {
		return gomel.NewComplianceError("Can not retrieve unit's self-predecessor")
	}
	if selfPredecessor.Creator() != u.Creator() {
		return gomel.NewComplianceError("Not descendant of predecessor")
	}
	return nil
}
