package gomel

// DecodeErrorHandler is a function that processes errors encountered while decoding parents of a preunit.
// If it succeeds in recovering from error, it should return a valid list of parents and nil.
type DecodeErrorHandler func(error) ([]Unit, error)

// CheckErrorHandler is a function that processes errors encountered while checking a newly built unit.
// If it cannot process a particular error, it should return it for further handling.
type CheckErrorHandler func(error) error

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddOwnUnit adds to the dag a unit produced by creating service (blocks until unit is added).
	AddOwnUnit(Preunit) Unit
	// AddUnit adds a single unit received from the given process to the underlying dag.
	AddUnit(Preunit, uint16) error
	// AddUnits adds multiple units received from the given process to the underlying dag.
	AddUnits([]Preunit, uint16) *AggregateError
	// AddDecodeErrorHandler adds new error handler for processing errors encountered during decoding preunit's parents.
	AddDecodeErrorHandler(DecodeErrorHandler)
	// AddCheckErrorHandler adds new error handler for processing errors encountered during checks.
	AddCheckErrorHandler(CheckErrorHandler)
}
