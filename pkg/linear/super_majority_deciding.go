package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type superMajorityDecider struct {
	*unanimousVoter
	decision      vote
	decisionLevel int
}

func newSuperMajorityDecider(
	uc gomel.Unit,
	dag gomel.Dag,
	rs gomel.RandomSource,
	commonVoteDeterministicPrefix int,
	zeroVoteRoundForCommonVote int,
) *superMajorityDecider {

	voter := newUnanimousVoter(uc, dag, rs, commonVoteDeterministicPrefix, zeroVoteRoundForCommonVote)
	return &superMajorityDecider{unanimousVoter: voter, decision: undecided, decisionLevel: -1}
}

// DecideUnitIsPopular decides if smd.uc is popular (i.e. it can be used as a timing unit).
// Returns vote, level on which the decision was made and current dag level.
func (smd *superMajorityDecider) DecideUnitIsPopular(dagMaxLevel int) (decision vote, decisionLevel int) {
	if smd.decision != undecided {
		return smd.decision, decisionLevel
	}
	maxDecisionLevel := smd.getMaxDecideLevel(dagMaxLevel)

	for level := smd.uc.Level() + firstVotingRound + 1; level <= maxDecisionLevel; level++ {
		decision := undecided

		commonVote := smd.lazyCommonVote(level)
		smd.dag.UnitsOnLevel(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				vDecision := smd.decide(v)
				if vDecision != undecided && vDecision == commonVote() {
					decision = vDecision
					return false
				}
			}
			return true
		})

		if decision != undecided {
			smd.decision = decision
			smd.decisionLevel = level
			smd.unanimousVoter.dispose()
			return decision, level
		}
	}

	return undecided, -1
}

func (smd *superMajorityDecider) decide(u gomel.Unit) vote {
	commonVote := smd.lazyCommonVote(u.Level() - 1)
	var votingResult votingResult
	result := voteUsingPrimeAncestors(smd.uc, u, smd.dag, func(uc, uPrA gomel.Unit) (vote vote, finish bool) {
		result := smd.VoteUsing(uPrA)
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

// getMaxDecideLevel returns a maximal level of a prime unit which can be used for deciding assuming that dag is on level
// 'dagMaxLevel'.
func (smd *superMajorityDecider) getMaxDecideLevel(dagMaxLevel int) int {
	deterministicLevel := smd.uc.Level() + int(smd.commonVoteDeterministicPrefix)
	if dagMaxLevel-2 < deterministicLevel {
		if deterministicLevel > dagMaxLevel {
			return dagMaxLevel
		}
		return deterministicLevel
	}
	return dagMaxLevel - 2
}
