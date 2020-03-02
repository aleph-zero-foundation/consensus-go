package linear

import (
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// ordering is a type implementing the ordering algorithm of units for a given dag.
type ordering struct {
	deciders                      map[gomel.Hash]*superMajorityDecider
	dag                           gomel.Dag
	randomSource                  gomel.RandomSource
	lastTUs                       []gomel.Unit
	currentTU                     gomel.Unit
	lastDecideResult              bool
	firstRoundZeroForCommonVote   int
	firstDecidingRound            int
	orderStartLevel               int
	crpFixedPrefix                uint16
	commonVoteDeterministicPrefix int
	log                           zerolog.Logger
}

// newOrdering constructs an iterator like object that is responsible of ordering units in a given dag.
func newOrdering(dag gomel.Dag, rs gomel.RandomSource, conf config.Config, log zerolog.Logger) *ordering {
	return &ordering{
		dag:                           dag,
		randomSource:                  rs,
		deciders:                      make(map[gomel.Hash]*superMajorityDecider),
		lastTUs:                       make([]gomel.Unit, conf.FirstRoundZeroForCommonVote),
		firstRoundZeroForCommonVote:   conf.FirstRoundZeroForCommonVote,
		firstDecidingRound:            conf.FirstDecidingRound,
		orderStartLevel:               conf.OrderStartLevel,
		crpFixedPrefix:                conf.CRPFixedPrefix,
		commonVoteDeterministicPrefix: conf.CommonVoteDeterministicPrefix,
		log:                           log,
	}
}

// NextRound tries to pick the next timing unit. Returns nil if it cannot be decided yet.
func (ord *ordering) NextRound() *timingRound {
	if ord.lastDecideResult {
		ord.lastDecideResult = false
	}

	dagMaxLevel := dagMaxLevel(ord.dag)
	if dagMaxLevel < ord.orderStartLevel {
		return nil
	}

	level := ord.orderStartLevel
	if ord.currentTU != nil {
		level = ord.currentTU.Level() + 1
	}
	if dagMaxLevel < level+ord.firstDecidingRound {
		return nil
	}

	previousTU := ord.currentTU
	decided := false
	ord.crpIterate(level, previousTU, func(uc gomel.Unit) bool {
		decider := ord.getDecider(uc)
		decision, decidedOn := decider.DecideUnitIsPopular(dagMaxLevel)
		if decision == popular {
			ord.log.
				Info().
				Int(logging.Height, decidedOn).
				Int(logging.Size, dagMaxLevel).
				Int(logging.Round, level).
				Msg(logging.NewTimingUnit)

			ord.lastTUs = ord.lastTUs[1:]
			ord.lastTUs = append(ord.lastTUs, ord.currentTU)
			ord.currentTU = uc
			ord.lastDecideResult = true
			ord.deciders = make(map[gomel.Hash]*superMajorityDecider)

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
	return newTimingRound(ord.currentTU, ord.lastTUs)
}

func (ord *ordering) getDecider(uc gomel.Unit) *superMajorityDecider {
	var decider *superMajorityDecider
	decider = ord.deciders[*uc.Hash()]
	if decider == nil {
		decider = newSuperMajorityDecider(
			uc,
			ord.dag,
			ord.randomSource,
			ord.commonVoteDeterministicPrefix,
			ord.firstRoundZeroForCommonVote,
		)
		ord.deciders[*uc.Hash()] = decider
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
