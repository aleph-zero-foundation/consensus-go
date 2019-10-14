package tests

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

type adder struct {
	dag gomel.Dag
}

// NewAdder creates a very simple adder for testing purposes.
func NewAdder(dag gomel.Dag) gomel.Adder {
	return &adder{dag}
}

func (a *adder) AddUnit(pu gomel.Preunit) error {
	_, err := gomel.AddUnit(a.dag, pu)
	return err
}

func (a *adder) AddAntichain(pus []gomel.Preunit) *gomel.AggregateError {
	result := make([]error, len(pus))
	for i, pu := range pus {
		_, result[i] = gomel.AddUnit(a.dag, pu)
	}
	return gomel.NewAggregateError(result)
}

func (a *adder) Register(dag gomel.Dag) {
	a.dag = dag
}
