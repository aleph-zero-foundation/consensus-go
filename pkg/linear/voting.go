package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type vote int

const (
	popular vote = iota
	unpopular
	undecided
)

type votingResult struct {
	popular   uint64
	unpopular uint64
}

// Deterministic function of a unit and level
// It is implemented as level-th bit of unit hash
// return 1 or 0
func simpleCoin(u gomel.Unit, level int) bool {
	index := level % (8 * len(u.Hash()))
	byteIndex, bitIndex := index/8, index%8
	return u.Hash()[byteIndex]&(1<<uint(bitIndex)) > 0
}

// Checks if votes for popular or unpopular make a quorum.
// Returns the vote making a quorum or undecided if there is no quorum.
func superMajority(dag gomel.Dag, votes votingResult) vote {
	if dag.IsQuorum(int(votes.popular)) {
		return popular
	}
	if dag.IsQuorum(int(votes.unpopular)) {
		return unpopular
	}
	return undecided
}

// Decides if uc is popular (i.e. it can be used as a timing unit).
// Returns vote, level on which the decision was made and current dag level.
func (o *ordering) decideUnitIsPopular(uc gomel.Unit, dagLevelReached int) (decision vote, decisionLevel int, dagLevel int) {
	decider, maxDecisionLevel := o.deciderGovernor.initializeDecider(uc, dagLevelReached)

	for level := uc.Level() + decidingRound; level <= maxDecisionLevel; level++ {
		decision := undecided

		o.dag.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if curDecision := decider.decide(uc, v); curDecision != undecided {
					decision = curDecision
					return false
				}

			}
			return true
		})

		if decision != undecided {
			return decision, level, dagLevelReached
		}
	}

	return undecided, -1, dagLevelReached
}

func voteUsingPrimeAncestors(uc, u gomel.Unit, dag gomel.Dag, voter func(uc, u gomel.Unit) (vote vote, finish bool)) (votesLevelBelow votingResult) {
	dag.PrimeUnits(u.Level() - 1).Iterate(func(primes []gomel.Unit) bool {
		votesOne := false
		votesZero := false
		finish := false
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			vote := undecided
			vote, finish = voter(uc, v)
			if vote == popular {
				votesOne = true
			} else if vote == unpopular {
				votesZero = true
			}
			if finish || (votesOne && votesZero) {
				break
			}
		}
		if votesOne {
			votesLevelBelow.popular++
		}
		if votesZero {
			votesLevelBelow.unpopular++
		}
		if finish {
			return false
		}
		return true
	})
	return votesLevelBelow
}
