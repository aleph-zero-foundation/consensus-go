package gomel

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddUnit to the underlying dag. Waits until the adding finishes and returns an error if applicable.
	AddUnit(Preunit) error
	// AddAntichain to the underlying dag. Waits until the adding of all units finishes and
	// returns the AggregateError with errors corresponding to the respecitive preunits.
	AddAntichain([]Preunit) *AggregateError
}

// AddUnit to the specified dag.
func AddUnit(dag Dag, pu Preunit) (Unit, error) {
	result, err := dag.Decode(pu)
	if err != nil {
		return nil, err
	}
	err = dag.Check(result)
	if err != nil {
		return nil, err
	}
	return dag.Emplace(result), nil
}
