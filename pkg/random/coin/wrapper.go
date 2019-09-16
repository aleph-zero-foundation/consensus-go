package coin

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type checkAndUpdate struct {
	gomel.Dag
	c *coin
}

func (dag *checkAndUpdate) Check(u gomel.Unit) error {
	if err := dag.Dag.Check(u); err != nil {
		return err
	}
	return dag.c.checkCompliance(u)
}

func (dag *checkAndUpdate) Emplace(u gomel.Unit) gomel.Unit {
	dag.c.update(u)
	return dag.Dag.Emplace(u)
}

func checkAndUpdateWrapper(dag gomel.Dag, c *coin) gomel.Dag {
	return &checkAndUpdate{dag, c}
}
