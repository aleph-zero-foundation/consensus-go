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
	pid                           uint16
	dag                           gomel.Dag
	randomSource                  gomel.RandomSource
	deciders                      map[gomel.Hash]*superMajorityDecider
	output                        chan<- []gomel.Unit
	trigger                       chan struct{}
	timingRounds                  chan *timingRound
	lastTUs                       []gomel.Unit
	currentTU                     gomel.Unit
	lastDecideResult              bool
	firstRoundZeroForCommonVote   int
	firstDecidingRound            int
	orderStartLevel               int
	crpFixedPrefix                uint16
	commonVoteDeterministicPrefix int
	wg                            sync.WaitGroup
	log                           zerolog.Logger
}

// NewExtender constructs an extender working on the given dag and sending rounds of ordered units to the given output.
func NewExtender(dag gomel.Dag, rs gomel.RandomSource, conf config.Config, output chan<- []gomel.Unit, log zerolog.Logger) *Extender {
	ext := &Extender{
		pid:                           conf.Pid,
		dag:                           dag,
		randomSource:                  rs,
		deciders:                      make(map[gomel.Hash]*superMajorityDecider),
		output:                        output,
		trigger:                       make(chan struct{}, 1),
		timingRounds:                  make(chan *timingRound, 10),
		lastTUs:                       make([]gomel.Unit, conf.FirstRoundZeroForCommonVote),
		firstRoundZeroForCommonVote:   conf.FirstRoundZeroForCommonVote,
		firstDecidingRound:            conf.FirstDecidingRound,
		orderStartLevel:               conf.OrderStartLevel,
		crpFixedPrefix:                conf.CRPFixedPrefix,
		commonVoteDeterministicPrefix: conf.CommonVoteDeterministicPrefix,
		log:                           log.With().Int(logging.Service, logging.ExtenderService).Logger(),
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
		round := ext.NextRound()
		for round != nil {
			ext.timingRounds <- round
			round = ext.NextRound()
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

func (ext *Extender) getDecider(uc gomel.Unit) *superMajorityDecider {
	var decider *superMajorityDecider
	decider = ext.deciders[*uc.Hash()]
	if decider == nil {
		decider = newSuperMajorityDecider(uc, ext.dag, ext.randomSource, ext.commonVoteDeterministicPrefix, ext.firstRoundZeroForCommonVote)
		ext.deciders[*uc.Hash()] = decider
	}
	return decider
}

// NextRound tries to pick the next timing unit. Returns nil if it cannot be decided yet.
func (ext *Extender) NextRound() *timingRound {
	if ext.lastDecideResult {
		ext.lastDecideResult = false
	}

	dagMaxLevel := dagMaxLevel(ext.dag)
	if dagMaxLevel < ext.orderStartLevel {
		return nil
	}

	level := ext.orderStartLevel
	if ext.currentTU != nil {
		level = ext.currentTU.Level() + 1
	}
	if dagMaxLevel < level+ext.firstDecidingRound {
		return nil
	}

	previousTU := ext.currentTU
	decided := false
	ext.crpIterate(level, previousTU, func(uc gomel.Unit) bool {
		decider := ext.getDecider(uc)
		decision, decidedOn := decider.DecideUnitIsPopular(dagMaxLevel)
		if decision == popular {
			ext.log.Info().Int(logging.Height, decidedOn).Int(logging.Size, dagMaxLevel).Int(logging.Round, level).Msg(logging.NewTimingUnit)

			ext.lastTUs = ext.lastTUs[1:]
			ext.lastTUs = append(ext.lastTUs, ext.currentTU)
			ext.currentTU = uc
			ext.deciders = nil
			ext.lastDecideResult = true
			ext.deciders = make(map[gomel.Hash]*superMajorityDecider)

			decided = true
			return false
		}
		if decision == undecided {
			return false
		}
		return true
	})
	if !decided {
		return nil
	}
	return newTimingRound(ext.currentTU, ext.lastTUs)
}

// dagMaxLevel returns the maximal level of a unit in the dag.
func dagMaxLevel(dag gomel.Dag) int {
	maxLevel := -1
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if v.Level() > maxLevel {
				maxLevel = v.Level()
			}
		}
		return true
	})
	return maxLevel
}
