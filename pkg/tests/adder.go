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
	result := make([]error, len(pus))
	noErrors := true
	for i, pu := range pus {
		if pu.EpochID() != ad.dag.EpochID() {
			result[i] = gomel.NewDataError("wrong epoch")
			noErrors = false
			continue
		}
		alreadyInDag := ad.dag.GetUnit(pu.Hash())
		if alreadyInDag != nil {
			result[i] = gomel.NewDuplicateUnit(alreadyInDag)
			noErrors = false
			continue
		}
		parents, err := ad.dag.DecodeParents(pu)
		if err != nil {
			result[i] = err
			noErrors = false
			continue
		}
		freeUnit := ad.dag.BuildUnit(pu, parents)
		err = ad.dag.Check(freeUnit)
		if err != nil {
			result[i] = err
			noErrors = false
			continue
		}
		ad.dag.Insert(freeUnit)
	}
	if noErrors {
		return nil
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
