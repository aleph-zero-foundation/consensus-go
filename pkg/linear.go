package gomel

// LinearOrdering is an interface for establishing a linear order of units.
type LinearOrdering interface {
	// AttemptTimingDecision chooses as many new timing units as possible and returns the level of the highest timing unit chosen so far.
	AttemptTimingDecision() int
	// TimingRound returns all units in timing round r, that is all units that are below r-th timing unit but not below (r-1)-th timing unit.
	// If any of those timing units has not yet been chosen, an error is returned.
	TimingRound(int) ([]Unit, error)
}
