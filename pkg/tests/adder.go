package tests

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

type adder struct {
	dag gomel.Dag
}

// NewAdder creates a very simple adder for testing purposes.
func NewAdder(dag gomel.Dag) gomel.Adder {
	return &adder{dag}
}

func (ad *adder) Close() {}

func (ad *adder) AddPreunits(source uint16, pus ...gomel.Preunit) []error {
	var result []error
	getErrors := func() []error {
		if result == nil {
			result = make([]error, len(pus))
		}
		return result
	}
	for i, pu := range pus {
		if pu.EpochID() != ad.dag.EpochID() {
			getErrors()[i] = gomel.NewDataError("wrong epoch")
			continue
		}
		alreadyInDag := ad.dag.GetUnit(pu.Hash())
		if alreadyInDag != nil {
			getErrors()[i] = gomel.NewDuplicateUnit(alreadyInDag)
			continue
		}
		parents, err := ad.dag.DecodeParents(pu)
		if err != nil {
			getErrors()[i] = err
			continue
		}
		freeUnit := ad.dag.BuildUnit(pu, parents)
		err = ad.dag.Check(freeUnit)
		if err != nil {
			getErrors()[i] = err
			continue
		}
		ad.dag.Insert(freeUnit)
	}
	return result
}

// AddUnit adds a preunit to the given dag.
func AddUnit(dag gomel.Dag, pu gomel.Preunit) (gomel.Unit, error) {
	err := NewAdder(dag).AddPreunits(pu.Creator(), pu)
	if err != nil && err[0] != nil {
		return nil, err[0]
	}
	return dag.GetUnit(pu.Hash()), nil
}
