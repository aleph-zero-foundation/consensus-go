package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Orderer is a service for sorting units in linear order
// - orderingRequests is an external channel from which we read that we should run attemptTimingDecision
// - orderedUnits is an output channel where we write units in linear order
// - extendOrderRequests is a channel used by the go routine which is choosing timing units to inform
//   the go routine which is ordering units that new timingUnits has been chosen
// - statistics is an external channel where we write number of units in consecutive rounds
// - currentRound is the round up to which we have chosen timing units
type Orderer struct {
	linearOrdering      gomel.LinearOrdering
	orderingRequests    chan struct{}
	extendOrderRequests chan [2]int
	orderedUnits        chan gomel.Unit
	statistics          chan int
	currentRound        int
}

// NewOrderer is a constructor of an ordering service
func NewOrderer(linearOrdering gomel.LinearOrdering, orderingRequests chan struct{}, orderedUnits chan gomel.Unit, statistics chan int) *Orderer {
	return &Orderer{
		linearOrdering:      linearOrdering,
		orderingRequests:    orderingRequests,
		extendOrderRequests: make(chan [2]int),
		orderedUnits:        orderedUnits,
		statistics:          statistics,
		currentRound:        0,
	}
}

func (o *Orderer) attemptOrdering() {
	for range o.orderingRequests {
		round := o.linearOrdering.AttemptTimingDecision()
		if round > o.currentRound {
			o.extendOrderRequests <- [2]int{o.currentRound, round}
			o.currentRound = round
		}
	}
	close(o.extendOrderRequests)
}

func (o *Orderer) extendOrder() {
	for rInterval := range o.extendOrderRequests {
		for r := rInterval[0]; r < rInterval[1]; r++ {
			units, _ := o.linearOrdering.TimingRound(r)
			o.statistics <- len(units)
			for _, u := range units {
				o.orderedUnits <- u
			}
		}
	}
	close(o.orderedUnits)
	close(o.statistics)
}

// Start is a function which starts the service
func (o *Orderer) Start() error {
	go o.attemptOrdering()
	go o.extendOrder()
	return nil
}

// Stop is the function that stops the service
func (o *Orderer) Stop() {
	close(o.orderingRequests)
}
