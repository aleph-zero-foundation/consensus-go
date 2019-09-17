package random

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type wrapper struct {
	gomel.Dag
	checkCompliance func(gomel.Unit) error
	update          func(gomel.Unit)
}

func (dag *wrapper) Check(u gomel.Unit) error {
	if err := dag.Dag.Check(u); err != nil {
		return err
	}
	return dag.checkCompliance(u)
}

func (dag *wrapper) Emplace(u gomel.Unit) gomel.Unit {
	dag.update(u)
	return dag.Dag.Emplace(u)
}

// Wrap the dag to force it to be usable with a random source.
func Wrap(dag gomel.Dag, checkCompliance func(gomel.Unit) error, update func(gomel.Unit)) gomel.Dag {
	return &wrapper{
		Dag:             dag,
		checkCompliance: checkCompliance,
		update:          update,
	}
}
