// Package check implements wrappers for dags that validate whether units respect predefined rules.
package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type basicCompliance struct {
	gomel.Dag
}

// BasicCompliance returns a version of the dag that will check the following notion of correctness:
//  1. If a unit has 0 parents and is a dealing unit it is correct, otherwise
//  2. A unit has to have at least two parents.
//  3. A unit has to have a predecessor with the same creator.
func BasicCompliance(dag gomel.Dag) gomel.Dag {
	return &basicCompliance{dag}
}

func (dag *basicCompliance) Check(u gomel.Unit) error {
	if err := dag.Dag.Check(u); err != nil {
		return err
	}
	return checkBasicCorrectness(u)
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
