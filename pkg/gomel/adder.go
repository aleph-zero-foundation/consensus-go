package gomel

// ErrorHandler is a function that processes errors encountered while checking a newly built unit.
// The last argument is the process ID who sent us that unit.
// If it cannot process a particular error, it should return it for further handling.
type ErrorHandler func(error, Unit, uint16) error

// RequestGossip is a function that initializes gossip with the given PID.
type RequestGossip func(uint16)

// RequestFetch is a function that contacts the given PID and requests units with given IDs.
type RequestFetch func(uint16, []uint64)

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddUnit adds a single unit received from the given process to the underlying dag.
	AddUnit(Preunit, uint16) error
	// AddUnits adds multiple units received from the given process to the underlying dag.
	AddUnits([]Preunit, uint16) *AggregateError
	// AddErrorHandler adds new error handler for processing errors encountered during checks.
	AddErrorHandler(ErrorHandler)
	// SetGossip passes to adder a function to trigger gossip.
	SetGossip(RequestGossip)
	// SetFetch passes to adder a function to trigger fetch.
	SetFetch(RequestFetch)
}
