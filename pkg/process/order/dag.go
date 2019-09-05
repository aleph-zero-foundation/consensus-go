package order

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type orderDag struct {
	gomel.Dag
	primeAlert chan<- struct{}
}

func (dag *orderDag) AddUnit(pu gomel.Preunit, callback gomel.Callback) {
	gomel.AddUnit(dag, pu, callback)
}

func (dag *orderDag) Emplace(u gomel.Unit) gomel.Unit {
	result := dag.Dag.Emplace(u)
	if gomel.Prime(result) {
		select {
		case dag.primeAlert <- struct{}{}:
		default:
		}
	}
	return result
}
