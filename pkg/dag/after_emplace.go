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
