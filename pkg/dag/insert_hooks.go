package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type afterInsert struct {
	gomel.Dag
	handle func(gomel.Unit)
}

// AfterInsert wraps the dag to call handle on the result of every Insert.
func AfterInsert(dag gomel.Dag, handle func(gomel.Unit)) gomel.Dag {
	return &afterInsert{dag, handle}
}

func (ae *afterInsert) Insert(u gomel.Unit) {
	ae.Dag.Insert(u)
	ae.handle(u)
}

type beforeInsert struct {
	gomel.Dag
	handle func(gomel.Unit)
}

// BeforeInsert wraps the dag to call handle on the result of every Insert.
func BeforeInsert(dag gomel.Dag, handle func(gomel.Unit)) gomel.Dag {
	return &beforeInsert{dag, handle}
}

func (be *beforeInsert) Insert(u gomel.Unit) {
	be.handle(u)
	be.Dag.Insert(u)
}
