package gomel

// LinearOrdering is an interface for establishing a linear order of units.
type LinearOrdering interface {
	// NextRound tries to pick a next timing unit and returns a suspended computation responsible for ordering units
	// for that round. Returns nil if it cannot be decided yet.
	NextRound() TimingRound
}

// TimingRound represents a particular round of voting and associated ordering of units.
type TimingRound interface {
	// TimingUnit returns a timing unit selected for this round.
	TimingUnit() Unit
	// OrderedUnits establishes the linear ordering of the units in this timing round and returns them.
	OrderedUnits() []Unit
}
