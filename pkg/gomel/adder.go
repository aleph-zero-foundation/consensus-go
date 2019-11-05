package gomel

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddUnit adds a single unit to the underlying dag. Waits until the adding finishes and returns an error if applicable.
	AddUnit(Preunit) error
	// AddUnits adds multiple units to the underlying . Waits until the adding of all units finishes and
	// returns the AggregateError with errors corresponding to the respective preunits.
	AddUnits([]Preunit) *AggregateError
	// Register passes the Dag to which units will be added. Must be called before calling any of adding methods.
	Register(Dag)
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
