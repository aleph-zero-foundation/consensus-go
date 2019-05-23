package gomel

// LinearOrdering is an interface for establishing a linear order of units.
type LinearOrdering interface {
	// TimingRound establishes the linear ordering on the units in timing round r and returns them.
	// If the timing decision has not yet been taken it returns nil.
	TimingRound(int) []Unit
	// DecideTimingOnLevel tries to pick a timing unit on a given level. Returns nil if it cannot be decided yet.
	DecideTimingOnLevel(int) Unit
}
