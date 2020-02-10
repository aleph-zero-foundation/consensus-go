// Package check implements wrappers for dags that validate whether units respect predefined rules.
package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// NoLevelSkipping ensures that no process creates a unit of level n>0 without creating a unit of level n-1.
// To check that it is sufficient to test whether height = level for every unit.
func NoLevelSkipping(u gomel.Unit, _ gomel.Dag) error {
	if u.Level() != u.Height() {
		return gomel.NewComplianceError("the level of the unit is different than its height")
	}
	return nil
}

// NoForks ensures that forked units are not added to the dag.
func NoForks(u gomel.Unit, dag gomel.Dag) error {
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
