package linear

import (
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Extender is a type that implements an algorithm that extends order of units provided by an instance of a Dag to a linear order.
type Extender struct {
	deciders                      map[gomel.Hash]*superMajorityDecider
	dag                           gomel.Dag
	randomSource                  gomel.RandomSource
	lastTUs                       []gomel.Unit
	currentTU                     gomel.Unit
	lastDecideResult              bool
	zeroVoteRoundForCommonVote    int
	firstDecidingRound            int
	orderStartLevel               int
	commonVoteDeterministicPrefix int
	crpIterator                   *CommonRandomPermutation
	log                           zerolog.Logger
}

// NewExtender constructs an iterator like object that is responsible of ordering units in a given dag.
func NewExtender(dag gomel.Dag, rs gomel.RandomSource, conf config.Config, log zerolog.Logger) *Extender {
	return &Extender{
		dag:                           dag,
		randomSource:                  rs,
		deciders:                      make(map[gomel.Hash]*superMajorityDecider),
		lastTUs:                       make([]gomel.Unit, conf.ZeroVoteRoundForCommonVote),
		zeroVoteRoundForCommonVote:    conf.ZeroVoteRoundForCommonVote,
		firstDecidingRound:            conf.FirstDecidingRound,
		orderStartLevel:               conf.OrderStartLevel,
		commonVoteDeterministicPrefix: conf.CommonVoteDeterministicPrefix,
		crpIterator:                   NewCommonRandomPermutation(dag, rs, conf.CRPFixedPrefix),
		log:                           log,
	}
}

// NextRound tries to pick the next timing unit. Returns nil if it cannot be decided yet.
func (ext *Extender) NextRound() *TimingRound {
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
	randomBytesPresent := ext.crpIterator.CRPIterate(level, previousTU, func(uc gomel.Unit) bool {
		decider := ext.getDecider(uc)
		decision, decidedOn := decider.DecideUnitIsPopular(dagMaxLevel)
		if decision == popular {
			ext.log.Info().Int(lg.Height, decidedOn).Int(lg.Size, dagMaxLevel).Int(lg.Round, level).Msg(lg.NewTimingUnit)
			ext.lastTUs = ext.lastTUs[1:]
			ext.lastTUs = append(ext.lastTUs, ext.currentTU)
			ext.currentTU = uc
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
	if !randomBytesPresent {
		ext.log.Debug().Int(lg.Round, level).Msg(lg.MissingRandomBytes)
	}
	if !decided {
		return nil
	}
	return newTimingRound(ext.currentTU, ext.lastTUs)
}

func (ext *Extender) getDecider(uc gomel.Unit) *superMajorityDecider {
	var decider *superMajorityDecider
	decider = ext.deciders[*uc.Hash()]
	if decider == nil {
		decider = newSuperMajorityDecider(
			uc,
			ext.dag,
			ext.randomSource,
			ext.commonVoteDeterministicPrefix,
			ext.zeroVoteRoundForCommonVote,
		)
		ext.deciders[*uc.Hash()] = decider
	}
	return decider
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
