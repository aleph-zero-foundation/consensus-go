package tests

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

type adder struct {
	dag gomel.Dag
}

// NewAdder creates a very simple adder for testing purposes.
func NewAdder(dag gomel.Dag) gomel.Adder {
	return &adder{dag}
}
func (ad *adder) AddDecodeErrorHandler(gomel.DecodeErrorHandler) {}
func (ad *adder) AddCheckErrorHandler(gomel.CheckErrorHandler)   {}

func (ad *adder) AddUnit(pu gomel.Preunit, source uint16) error {
	parents, err := ad.dag.DecodeParents(pu)
	if err != nil {
		return err
	}
	freeUnit := ad.dag.BuildUnit(pu, parents)
	err = ad.dag.Check(freeUnit)
	if err != nil {
		return err
	}
	unitInDag := ad.dag.Transform(freeUnit)
	ad.dag.Insert(unitInDag)
	return nil
}

func (ad *adder) AddUnits(pus []gomel.Preunit, source uint16) *gomel.AggregateError {
	result := make([]error, len(pus))
	for i, pu := range pus {
		result[i] = ad.AddUnit(pu, source)
	}
	return gomel.NewAggregateError(result)
}
