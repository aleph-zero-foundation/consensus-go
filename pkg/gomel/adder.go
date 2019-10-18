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
func AddUnit(dag Dag, unit Unit) error {
	result, err := dag.Prepare(unit)
	if err != nil {
		return err
	}
	dag.Insert(result)
	return nil
}
