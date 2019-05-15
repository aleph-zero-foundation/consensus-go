package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Orderer is a service for sorting units in linear order
// - attemptTimingRequests is an external channel from which we read that we should run attemptTimingDecision
// - orderedUnits is an output channel where we write units in linear order
// - extendOrderRequests is a channel used by the go routine which is choosing timing units to inform
//   the go routine which is ordering units that new timingUnit has been chosen
// - statistics is an external channel where we write number of units in consecutive rounds
// - currentRound is the round up to which we have chosen timing units
type Orderer struct {
	linearOrdering        gomel.LinearOrdering
	attemptTimingRequests <-chan struct{}
	extendOrderRequests   chan int
	orderedUnits          chan<- gomel.Unit
	statistics            chan<- int
	currentRound          int
	exitChan              chan struct{}
}

// NewOrderer is a constructor of an ordering service
func NewOrderer(linearOrdering gomel.LinearOrdering, attemptTimingRequests <-chan struct{}, orderedUnits chan<- gomel.Unit, statistics chan<- int) *Orderer {
	return &Orderer{
		linearOrdering:        linearOrdering,
		attemptTimingRequests: attemptTimingRequests,
		extendOrderRequests:   make(chan int, 10),
		orderedUnits:          orderedUnits,
		statistics:            statistics,
		currentRound:          0,
		exitChan:              make(chan struct{}),
	}
}

func (o *Orderer) attemptOrdering() {
	for {
		select {
		case <-o.attemptTimingRequests:
			for o.linearOrdering.DecideTimingOnLevel(o.currentRound) != nil {
				o.extendOrderRequests <- o.currentRound
				o.currentRound++
			}
		case <-o.exitChan:
			close(o.extendOrderRequests)
			return
		}
	}
}

func (o *Orderer) extendOrder() {
	for round := range o.extendOrderRequests {
		units, _ := o.linearOrdering.TimingRound(round)
		o.statistics <- len(units)
		for _, u := range units {
			o.orderedUnits <- u
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
	close(o.exitChan)
}
