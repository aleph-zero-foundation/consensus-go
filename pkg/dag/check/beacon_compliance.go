// Package check implements wrappers for dags that validate whether units respect predefined rules.
package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// PrimeOnlyNoSkipping returns a version of the dag that checks whether every unit is a prime unit,
// and no process creates a unit of level n>0 without creating a unit of level n-1.
// To ensure that it is sufficient to check whether height = level for every unit.
func PrimeOnlyNoSkipping(dag gomel.Dag) gomel.Dag {
	return AddCheck(dag, checkPrimeOnlyNoSkipping)
}

func checkPrimeOnlyNoSkipping(u gomel.Unit) error {
	if u.Level() != u.Height() {
		return gomel.NewComplianceError("the level of the unit is different than its height")
	}
	return nil
}

type noForkUnit struct {
	gomel.Unit
}

func (u *noForkUnit) AboveWithinProc(v gomel.Unit) bool {
	return u.Creator() == v.Creator() && u.Height() >= v.Height()
}

// NoForks returns a dag that will error on adding a fork.
func NoForks(dag gomel.Dag) gomel.Dag {
	return AddCheckAndTransform(dag, func(u gomel.Unit) error {
		return checkNoForks(dag, u)
	}, func(u gomel.Unit) gomel.Unit {
		return &noForkUnit{u}
	})
}

func checkNoForks(dag gomel.Dag, u gomel.Unit) error {
	maxes := dag.MaximalUnitsPerProcess().Get(u.Creator())
	if len(maxes) == 0 {
		return nil
	}
	// There can be only one, because we don't allow forks.
	max := maxes[0]
	if max.Height() >= u.Height() {
		return gomel.NewComplianceError("the unit is a fork")
	}
	return nil
}
