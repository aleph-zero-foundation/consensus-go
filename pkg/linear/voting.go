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

// Vote of u on popularity of uc as described by the fast consensus algorithm
func newDefaultVote(votingLevel int, coinToss coinToss) defaultVote {
	return func(uc gomel.Unit, u gomel.Unit) vote {
		r := u.Level() - uc.Level() - votingLevel
		if r <= 0 {
			// "Default vote is asked on too low unit level."
			return undecided
		}
		if r == 1 {
			return popular
		}
		if r == 2 {
			return unpopular
		}
		if coinToss(uc, u) {
			return popular
		}
		return unpopular
	}
}

// Deterministic function of a unit and level
// It is implemented as level-th bit of unit hash
// return 1 or 0
func simpleCoin(u gomel.Unit, level int) bool {
	index := level % (8 * len(u.Hash()))
	byteIndex, bitIndex := index/8, index%8
	if u.Hash()[byteIndex]&(1<<uint(bitIndex)) > 0 {
		return true
	}
	return false
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

// coinToss at unit uTossing (necessarily at level >= ADD_SHARES + 1)
// With low probability the toss may fail -- typically because of adversarial behavior of some process(es).
// uc - the unit whose popularity decision is being considered by tossing a coin
//      this param is used only in case when the simpleCoin is used, otherwise
//      the result of coin toss is meant to be a function of uTossing.level() only
// uTossing - the unit that is tossing a coin
// returns: false or true -- a (pseudo)random bit, impossible to predict before (uTossing.level - 1) was reached
func newCoin(rs gomel.RandomSource) coinToss {
	return func(uc gomel.Unit, uTossing gomel.Unit) bool {
		level := uTossing.Level() - 1
		randomBytes := rs.RandomBytes(uTossing.Creator(), uTossing.Level()-1)
		if randomBytes == nil {
			return simpleCoin(uc, level)
		}
		return randomBytes[0]&1 == 0
	}
}

// Decides if uc is popular (i.e. it can be used as a timing unit)
// Returns vote, level on which the decision was made and current dag level
func (o *ordering) decideUnitIsPopular(uc gomel.Unit) (vote, int, int) {
	dagLevelReached := dagMaxLevel(o.dag)
	if result, ok := o.decisionMemo[*uc.Hash()]; ok {
		return result, -1, dagLevelReached
	}

	for level := uc.Level() + 1; level <= dagLevelReached; level++ {
		decision := undecided

		decisionMakers := o.decisionMakers

		o.dag.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				for _, decisionMaker := range decisionMakers {
					if curDecision := decisionMaker.decide(uc, v); curDecision != undecided {
						decision = curDecision
						return false
					}
				}
			}
			return true
		})

		if decision != undecided {
			o.decisionMemo[*uc.Hash()] = decision
			return decision, level, dagLevelReached
		}
	}

	return undecided, -1, dagLevelReached
}
