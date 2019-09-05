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
//  3. A unit has to have a predecessor.
//  4. A unit's first parent has to have the same creator as this unit.
//  5. A unit's first parent has to be its predecessor.
func BasicCompliance(dag gomel.Dag) gomel.Dag {
	return &basicCompliance{dag}
}

func (dag *basicCompliance) AddUnit(pu gomel.Preunit, callback gomel.Callback) {
	gomel.AddUnit(dag, pu, callback)
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
	firstParent := u.Parents()[0]
	if firstParent.Creator() != u.Creator() {
		return gomel.NewComplianceError("Not descendant of first parent")
	}
	// self-predecessor and the first unit on the Parents list should be equal
	if firstParent != selfPredecessor {
		return gomel.NewComplianceError("First parent of a unit is not equal to its self-predecessor")
	}

	return nil
}
