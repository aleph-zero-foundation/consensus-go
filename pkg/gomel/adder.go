package gomel

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddUnit to the underlying dag. Waits until the adding finishes and returns an error if applicable.
	AddUnit(Preunit) error
	// AddAntichain to the underlying dag. Waits until the adding of all units finishes and
	// returns the AggregateError with errors corresponding to the respective preunits.
	AddAntichain([]Preunit) *AggregateError
}

// AddUnit to the specified dag.
func AddUnit(dag Dag, pu Preunit) (Unit, error) {
	freeUnit, err := dag.Decode(pu)
	if err != nil {
		return nil, err
	}
	unitInDag, err := dag.Prepare(freeUnit)
	if err != nil {
		return nil, err
	}
	dag.Insert(unitInDag)
	return unitInDag, nil
}
