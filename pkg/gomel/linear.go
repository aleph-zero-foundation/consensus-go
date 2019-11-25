package gomel

// LinearOrdering is an interface for establishing a linear order of units.
type LinearOrdering interface {
	// DecideTiming tries to pick a next timing unit and returns a suspended computation responsible for ordering units
	// for that round. Returns nil if it cannot be decided yet.
	DecideTiming() TimingRound
}

// TimingRound represents a particular round of voting and associated ordering of units.
type TimingRound interface {
	// TimingUnit returns a timing unit selected for this round.
	TimingUnit() Unit
	// TimingRound establishes the linear ordering on the units in this timing round and returns them.
	TimingRound() []Unit
}
