// Package check implements wrappers for dags that validate whether units respect predefined rules.
package check

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type check func(gomel.Unit) error

type transform func(gomel.Unit) gomel.Unit

func identity() transform {
	return func(u gomel.Unit) gomel.Unit { return u }
}

type wrapper struct {
	gomel.Dag
	ch check
	tr transform
}

func (dag *wrapper) Prepare(u gomel.Unit) (gomel.Unit, error) {
	if err := dag.ch(u); err != nil {
		return nil, err
	}
	prep, err := dag.Dag.Prepare(u)
	if err != nil {
		return nil, err
	}
	return dag.tr(prep), nil
}

// AddCheck wraps the dag so that it performs the provided check on the units.
func AddCheck(dag gomel.Dag, ch check) gomel.Dag {
	return &wrapper{dag, ch, identity()}
}

// AddCheckAndTransform wraps the dag so that it performs the provided check and transform on the units.
func AddCheckAndTransform(dag gomel.Dag, ch check, tr transform) gomel.Dag {
	return &wrapper{dag, ch, tr}
}
