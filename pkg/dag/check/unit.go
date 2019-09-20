// Package check implements wrappers for dags that validate whether units respect predefined rules.
package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unitChecker struct {
	gomel.Dag
	check func(gomel.Unit) error
}

func (dag *unitChecker) Check(u gomel.Unit) error {
	if err := dag.Dag.Check(u); err != nil {
		return err
	}
	return dag.check(u)
}

// Units wraps the dag so that it performs the provided check on the units.
func Units(dag gomel.Dag, check func(gomel.Unit) error) gomel.Dag {
	return &unitChecker{dag, check}
}
