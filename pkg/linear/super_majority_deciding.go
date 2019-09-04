package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

const (
	firstDecidingRound = 3
)

type superMajorityDecider struct {
	*unanimousVoter
}

func newSuperMajorityDecider(dag gomel.Dag, rs gomel.RandomSource) *superMajorityDecider {
	vote := newUnanimousVoter(dag, rs)
	return &superMajorityDecider{vote}
}

// Decides if uc is popular (i.e. it can be used as a timing unit).
// Returns vote, level on which the decision was made and current dag level.
func (smd *superMajorityDecider) decideUnitIsPopular(uc gomel.Unit, dagMaxLevel int) (decision vote, decisionLevel int, dagLevel int) {
	maxDecisionLevel := smd.getMaximalLevelAtWhichWeCanDecide(uc, dagMaxLevel)

	for level := uc.Level() + firstDecidingRound; level <= maxDecisionLevel; level++ {
		decision := undecided

		commonVote := smd.lazyCommonVote(uc, level)
		smd.dag.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				vDecision := smd.decide(uc, v)
				if vDecision != undecided && vDecision == commonVote() {
					decision = vDecision
					return false
				}

			}
			return true
		})

		if decision != undecided {
			return decision, level, dagMaxLevel
		}
	}

	return undecided, -1, dagMaxLevel
}

func (smd *superMajorityDecider) decide(uc, u gomel.Unit) vote {
	commonVote := smd.lazyCommonVote(uc, u.Level()-1)
	var votingResult votingResult
	result := voteUsingPrimeAncestors(uc, u, smd.dag, func(uc, uPrA gomel.Unit) (vote vote, finish bool) {
		result := smd.vote(uc, uPrA)
		if result == undecided {
			result = commonVote()
		}
		updated := false
		switch result {
		case popular:
			votingResult.popular++
			updated = true
		case unpopular:
			votingResult.unpopular++
			updated = true
		}
		if updated {
			if superMajority(smd.dag, votingResult) != undecided {
				return result, true
			}
		} else {
			// fast fail
			test := votingResult
			remaining := smd.dag.NProc() - uPrA.Creator() - 1
			test.popular += remaining
			test.unpopular += remaining
			if superMajority(smd.dag, test) == undecided {
				return result, true
			}
		}

		return result, false
	})
	return superMajority(smd.dag, result)
}

func (smd *superMajorityDecider) getMaximalLevelAtWhichWeCanDecide(uc gomel.Unit, dagMaxLevel int) int {
	if dagMaxLevel-uc.Level()-2 < commonVoteDeterministicPrefix {
		return uc.Level() + commonVoteDeterministicPrefix
	}
	return dagMaxLevel - 2
}
