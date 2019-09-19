package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type afterEmplace struct {
	gomel.Dag
	handle func(gomel.Unit)
}

// AfterEmplace wraps the dag to call handle on the result of every Emplace.
func AfterEmplace(dag gomel.Dag, handle func(gomel.Unit)) gomel.Dag {
	return &afterEmplace{dag, handle}
}

func (ae *afterEmplace) Emplace(u gomel.Unit) gomel.Unit {
	result := ae.Dag.Emplace(u)
	ae.handle(result)
	return result
}

type beforeEmplace struct {
	gomel.Dag
	handle func(gomel.Unit)
}

// BeforeEmplace wraps the dag to call handle on the result of every Emplace.
func BeforeEmplace(dag gomel.Dag, handle func(gomel.Unit)) gomel.Dag {
	return &beforeEmplace{dag, handle}
}

func (be *beforeEmplace) Emplace(u gomel.Unit) gomel.Unit {
	be.handle(u)
	return be.Dag.Emplace(u)
}
