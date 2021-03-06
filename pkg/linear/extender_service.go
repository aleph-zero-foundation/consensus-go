// Package linear implements the algorithm for extending partial dag order into linear order.
package linear

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
)

// ExtenderService is a component working on a dag that extends a partial order of units defined by dag to a linear order.
// ExtenderService should be notified, by the means of its Notify method, when it should try to perform its task.
// If successful, ExtenderService collects all the units belonging to newest timing round, and sends them to the output channel.
type ExtenderService struct {
	ordering     *Extender
	pid          uint16
	output       chan<- []gomel.Unit
	trigger      chan struct{}
	finished     chan struct{}
	timingRounds chan *TimingRound
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewExtenderService constructs an extender working on the given dag and sending rounds of ordered units to the given output.
func NewExtenderService(dag gomel.Dag, rs gomel.RandomSource, conf config.Config, output chan<- []gomel.Unit, log zerolog.Logger) *ExtenderService {
	logger := log.With().Int(lg.Service, lg.ExtenderService).Logger()
	ordering := NewExtender(dag, rs, conf, logger)
	ext := &ExtenderService{
		ordering:     ordering,
		pid:          conf.Pid,
		output:       output,
		trigger:      make(chan struct{}, 1),
		finished:     make(chan struct{}),
		timingRounds: make(chan *TimingRound, conf.EpochLength),
		log:          logger,
	}

	ext.wg.Add(2)
	go ext.timingUnitDecider()
	go ext.roundSorter()
	ext.log.Info().Msg(lg.ServiceStarted)
	return ext
}

// Close stops the extender.
func (ext *ExtenderService) Close() {
	close(ext.finished)
	ext.wg.Wait()
	ext.log.Info().Msg(lg.ServiceStopped)
}

// Notify ExtenderService to attempt choosing next timing units.
func (ext *ExtenderService) Notify() {
	select {
	case ext.trigger <- struct{}{}:
	default:
	}
}

// timingUnitDecider tries to pick the next timing unit after receiving notification on trigger channel.
// For each picked timing unit, it sends a timingRound object to timingRounds channel.
func (ext *ExtenderService) timingUnitDecider() {
	defer ext.wg.Done()
	for {
		select {
		case <-ext.trigger:
			round := ext.ordering.NextRound()
			for round != nil {
				ext.timingRounds <- round
				round = ext.ordering.NextRound()
			}
		case <-ext.finished:
			close(ext.timingRounds)
			return
		}
	}
}

// roundSorter picks information about newly picked timing unit from the timingRounds channel,
// finds all units belonging to their timing round and establishes linear order on them.
// Sends slices of ordered units to output.
func (ext *ExtenderService) roundSorter() {
	defer ext.wg.Done()
	for round := range ext.timingRounds {
		units := round.OrderedUnits()
		ext.output <- units
		for _, u := range units {
			ext.log.Debug().Uint16(lg.Creator, u.Creator()).Int(lg.Height, u.Height()).Uint32(lg.Epoch, uint32(u.EpochID())).Msg(lg.UnitOrdered)
			if u.Creator() == ext.pid {
				ext.log.Info().Int(lg.Height, u.Height()).Int(lg.Level, u.Level()).Msg(lg.OwnUnitOrdered)
			}
		}
		ext.log.Info().Int(lg.Size, len(units)).Msg(lg.LinearOrderExtended)
	}
}
