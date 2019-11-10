package gomel

// DecodeErrorHandler is a function that processes errors encountered while decoding parents of a preunit.
// If it succeeds in recovering from error, it should return a valid list of parents and nil.
type DecodeErrorHandler func(error) ([]Unit, error)

// CheckErrorHandler is a function that processes errors encountered while checking a newly built unit.
// If it cannot process a particular error, it should return it for further handling.
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
