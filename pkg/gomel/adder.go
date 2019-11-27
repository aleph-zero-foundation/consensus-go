package gomel

// Adder represents a mechanism for adding units to a dag.
type Adder interface {
	// AddUnit adds a single unit received from the given process to the underlying dag.
	AddUnit(Preunit, uint16) error
	// AddUnits adds multiple units received from the given process to the underlying dag.
	AddUnits([]Preunit, uint16) *AggregateError
}
