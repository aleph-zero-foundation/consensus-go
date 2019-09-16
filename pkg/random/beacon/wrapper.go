package beacon

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type checkAndUpdate struct {
	gomel.Dag
	b *Beacon
}

func (dag *checkAndUpdate) Check(u gomel.Unit) error {
	if err := dag.Dag.Check(u); err != nil {
		return err
	}
	return dag.b.checkCompliance(u)
}

func (dag *checkAndUpdate) Emplace(u gomel.Unit) gomel.Unit {
	dag.b.update(u)
	return dag.Dag.Emplace(u)
}

func checkAndUpdateWrapper(dag gomel.Dag, b *Beacon) gomel.Dag {
	return &checkAndUpdate{dag, b}
}
