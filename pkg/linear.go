package gomel

// LinearOrdering is an abstract representation of a linear order being established on units.
type LinearOrdering interface {
	AttemptTimingDecision() int
	TimingRound(int) ([]Unit, error)
}
