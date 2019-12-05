package gomel

// ErrorHandler is a function that processes errors encountered while checking a newly built unit.
// The last argument is the process ID who sent us that unit.
// If it cannot process a particular error, it should return it for further handling.
type ErrorHandler func(error, Unit, uint16) error

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddUnit adds a single unit received from the given process to the underlying dag.
	AddUnit(Preunit, uint16) error
	// AddUnits adds multiple units received from the given process to the underlying dag.
	AddUnits([]Preunit, uint16) *AggregateError
	// AddErrorHandler adds new error handler for processing errors encountered during checks.
	AddErrorHandler(ErrorHandler)
}
