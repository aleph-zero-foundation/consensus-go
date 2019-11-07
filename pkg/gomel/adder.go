package gomel

// DecodeErrorHandler is a function that processes errors encountered while decoding parents of a preunit.
type DecodeErrorHandler func(error) ([]Unit, error)

// CheckErrorHandler is a function that processes errors encountered while checking a newly built unit.
type CheckErrorHandler func(error) error

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddUnit adds a single unit to the underlying dag. Waits until the adding finishes and returns an error if applicable.
	AddUnit(Preunit, uint16) error
	// AddUnits adds multiple units to the underlying . Waits until the adding of all units finishes and
	// returns the AggregateError with errors corresponding to the respective preunits.
	AddUnits([]Preunit, uint16) *AggregateError
	// AddDecodeErrorHandler adds new error handler for processing errors encountered during decoding preunit's parents.
	AddDecodeErrorHandler(DecodeErrorHandler)
	// AddCheckErrorHandler adds new error handler for processing errors encountered during checks.
	AddCheckErrorHandler(CheckErrorHandler)
}

/*
// AddUnit to the specified dag.
func AddUnit(dag Dag, pu Preunit, dehs []DecodeErrorHandler, cehs []CheckErrorHandler) (Unit, error) {
	parents, err := DecodeParents(dag, pu)
	if err != nil {
		for _, handler := range dehs {
			if parents, err = handler(err); err == nil {
				break
			}
		}
		if err != nil {
			return nil, err
		}
	}
	freeUnit := dag.BuildUnit(pu, parents)
	err = dag.Check(freeUnit)
	if err != nil {
		for _, handler := range cehs {
			if err = handler(err); err == nil {
				break
			}
		}
		if err != nil {
			return nil, err
		}
	}
	unitInDag := dag.Transform(freeUnit)
	dag.Insert(unitInDag)
	return unitInDag, nil
}
*/
