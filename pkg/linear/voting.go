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
	undecided uint64
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

// Checks if everyone voted for the same value.
func unanimousVoting(votingResult votingResult) vote {
	if votingResult.undecided > 0 {
		return undecided
	}
	if votingResult.popular > 0 && votingResult.unpopular == 0 {
		return popular
	}
	if votingResult.unpopular > 0 && votingResult.popular == 0 {
		return unpopular
	}
	return undecided
}

// Creates a new instance of coinToss using a given RandomSource.
// With low probability the toss may fail -- typically because of adversarial behavior of some process(es).
// uc - the unit whose popularity decision is being considered by tossing a coin
//      this param is used only in case when the simpleCoin is used, otherwise
//      the result of coin toss is meant to be a function of uTossing.level() only
// uTossing - the unit that is tossing a coin (necessarily at level >= ADD_SHARES + 1)
// returns: false or true -- a (pseudo)random bit, impossible to predict before (uTossing.level - 1) was reached
func newCoin(rs gomel.RandomSource) coinToss {
	return func(uc gomel.Unit, uTossing gomel.Unit) bool {
		level := uTossing.Level() - 1
		randomBytes := rs.RandomBytes(uTossing.Creator(), level)
		if randomBytes == nil {
			return simpleCoin(uc, level)
		}
		return randomBytes[0]&1 == 0
	}
}

// Decides if uc is popular (i.e. it can be used as a timing unit).
// Returns vote, level on which the decision was made and current dag level.
func (o *ordering) decideUnitIsPopular(uc gomel.Unit) (decision vote, decisionLevel int, dagLevel int) {
	dagLevelReached := dagMaxLevel(o.dag)
	if result, ok := o.decisionMemo[*uc.Hash()]; ok {
		return result, -1, dagLevelReached
	}

	for level := uc.Level() + 1; level <= dagLevelReached; level++ {
		decision := undecided

		o.dag.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if curDecision := o.decisionMaker.decide(uc, v); curDecision != undecided {
					decision = curDecision
					return false
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
