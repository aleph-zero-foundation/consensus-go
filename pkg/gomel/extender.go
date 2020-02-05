package gomel

// Extender extend a partial order to linear order and produces rounds of ordered units..
type Extender interface {
	// NextRound tries to pick a next timing unit and returns a suspended computation responsible for ordering units
	// for that round. Returns nil if it cannot be decided yet.
	NextRound() TimingRound
	// Close the extender. No further data will be accepted.
	Close()
}

// TimingRound represents a particular round of voting and associated ordering of units.
type TimingRound interface {
	// TimingUnit returns a timing unit selected for this round.
	TimingUnit() Unit
	// OrderedUnits establishes the linear ordering of the units in this timing round and returns them.
	OrderedUnits() []Unit
}
