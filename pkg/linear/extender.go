// Package linear implements the algorithm for extending partial dag order into linear order.
package linear

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Extender is a component working on a dag that extends a partial order of units defined by dag to a linear order.
// Extender reacts every time a new unit is inserted into the underlying dag. It tries to pick next timing unit.
// If successful, Extender collects all the units belonging to that timing round, and linearly orders them.
type Extender struct {
	ordering     *ordering
	pid          uint16
	output       chan<- []gomel.Unit
	trigger      chan struct{}
	timingRounds chan *timingRound
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewExtender constructs an extender working on the given dag and sending rounds of ordered units to the given output.
func NewExtender(dag gomel.Dag, rs gomel.RandomSource, conf config.Config, output chan<- []gomel.Unit, log zerolog.Logger) *Extender {
	logger := log.With().Int(logging.Service, logging.ExtenderService).Logger()
	ordering := newOrdering(dag, rs, conf, log)
	ext := &Extender{
		ordering:     ordering,
		pid:          conf.Pid,
		output:       output,
		trigger:      make(chan struct{}, 1),
		timingRounds: make(chan *timingRound, 10),
		log:          logger,
	}

	ext.wg.Add(2)
	go ext.timingUnitDecider()
	go ext.roundSorter()

	return ext
}

// Close stops the extender.
func (ext *Extender) Close() {
	close(ext.trigger)
	ext.wg.Wait()
}

// Notify Extender to attempt choosing next timing units.
func (ext *Extender) Notify() {
	select {
	case ext.trigger <- struct{}{}:
	default:
	}
}

// timingUnitDecider tries to pick the next timing unit after receiving notification on trigger channel.
// For each picked timing unit, it sends a timingRound object to timingRounds channel.
func (ext *Extender) timingUnitDecider() {
	defer ext.wg.Done()
	for range ext.trigger {
		round := ext.ordering.NextRound()
		for round != nil {
			ext.timingRounds <- round
			round = ext.ordering.NextRound()
		}
	}
	close(ext.timingRounds)
}

// roundSorter picks information about newly picked timing unit from the timingRounds channel,
// finds all units belonging to their timing round and establishes linear order on them.
// Sends slices of ordered units to output.
func (ext *Extender) roundSorter() {
	defer ext.wg.Done()
	for round := range ext.timingRounds {
		units := round.OrderedUnits()
		ext.output <- units
		for _, u := range units {
			ext.log.Info().
				Uint16(logging.Creator, u.Creator()).
				Int(logging.Height, u.Height()).
				Uint32(logging.Epoch, uint32(u.EpochID())).
				Msg(logging.UnitOrdered)
			if u.Creator() == ext.pid {
				ext.log.Info().Int(logging.Height, u.Height()).Msg(logging.OwnUnitOrdered)
			}
		}
		ext.log.Info().Int(logging.Size, len(units)).Msg(logging.LinearOrderExtended)
	}
}
