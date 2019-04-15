package linear

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Ordering is an implementation of LinearOrdering intended to work with a growing Poset.
type Ordering struct {
	poset gomel.Poset
}

// NewOrdering creates a ordering wrapper for the given poset.
func NewOrdering(poset gomel.Poset) gomel.LinearOrdering {
	return &Ordering{
		poset: poset,
	}
}

// AttemptTimingDecision picks as many timing units as possible and returns the level up to which the timing units are picked.
func (o *Ordering) AttemptTimingDecision() int {
	// TODO: implement
	return 0
}

// TimingRound returns all the units in timing round r. If the timing decision has not yet been taken it returns an error.
func (o *Ordering) TimingRound(r int) ([]gomel.Unit, error) {
	// TODO: implement
	return nil, nil
}
