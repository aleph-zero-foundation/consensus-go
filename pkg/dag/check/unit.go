// Package check implements wrappers for dags that validate whether units respect predefined rules.
package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type checker func(gomel.Unit) error

type transformer func(gomel.Unit) gomel.Unit

func identity() transformer {
	return func(u gomel.Unit) gomel.Unit { return u }
}

type wrapper struct {
	gomel.Dag
	check     checker
	transform transformer
}

func (dag *wrapper) Prepare(u gomel.Unit) (gomel.Unit, error) {
	if err := dag.check(u); err != nil {
		return nil, err
	}
	prep, err := dag.Dag.Prepare(u)
	if err != nil {
		return nil, err
	}
	return dag.transform(prep), nil
}

// AddCheck wraps the dag so that it performs the provided check on the units.
func AddCheck(dag gomel.Dag, check checker) gomel.Dag {
	return &wrapper{dag, check, identity()}
}

// AddCheckAndTransform wraps the dag so that it performs the provided check and transform on the units.
func AddCheckAndTransform(dag gomel.Dag, check checker, transform transformer) gomel.Dag {
	return &wrapper{dag, check, transform}
}
