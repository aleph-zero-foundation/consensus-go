package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

const (
	decidingRound       = 3
	votingRound         = 1
	deterministicPrefix = 10
)

type superMajorityDecider struct {
	*unanimousVoter
}

func newSuperMajorityDecider(dag gomel.Dag) *superMajorityDecider {
	vote := newUnanimousVoter(dag)
	return &superMajorityDecider{vote}
}

func (smd *superMajorityDecider) getMaxDecisionLevel(uc gomel.Unit, dagMaxLevelReached int) (maxAvailableLevel int) {
	if (dagMaxLevelReached - uc.Level()) <= deterministicPrefix {
		return dagMaxLevelReached
	}
	return (dagMaxLevelReached - 2)
}

// Decides if uc is popular (i.e. it can be used as a timing unit).
// Returns vote, level on which the decision was made and current dag level.
func (smd *superMajorityDecider) decideUnitIsPopular(uc gomel.Unit, dagMaxLevel int) (decision vote, decisionLevel int, dagLevel int) {
	maxDecisionLevel := smd.getMaximalLevelAtWhichWeCanDecide(uc, dagMaxLevel)

	for level := uc.Level() + decidingRound; level <= maxDecisionLevel; level++ {
		decision := undecided

		smd.dag.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if curDecision := smd.decide(uc, v); curDecision != undecided {
					decision = curDecision
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
	if uc.Level() >= u.Level() {
		return undecided
	}
	if u.Level()-uc.Level() < decidingRound {
		return undecided
	}
	commonVote := smd.lazyCommonVote(uc, u.Level(), smd.dag)
	result := smd.decideUsingSuperMajorityOfVotes(uc, u)
	if result != undecided && result == commonVote() {
		return result
	}
	return undecided
}

func (smd *superMajorityDecider) decideUsingSuperMajorityOfVotes(uc, u gomel.Unit) vote {
	commonVote := smd.lazyCommonVote(uc, u.Level()-1, smd.dag)
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
			remaining := uint64(smd.dag.NProc() - uPrA.Creator() - 1)
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
	if dagMaxLevel-uc.Level() <= deterministicPrefix {
		return dagMaxLevel
	}
	return dagMaxLevel - 2
}

type unanimousVoter struct {
	dag        gomel.Dag
	rs         gomel.RandomSource
	votingMemo map[[2]gomel.Hash]vote
}

func newUnanimousVoter(dag gomel.Dag) *unanimousVoter {
	return &unanimousVoter{
		dag:        dag,
		votingMemo: make(map[[2]gomel.Hash]vote),
	}
}

func (uv *unanimousVoter) vote(uc, u gomel.Unit) (result vote) {
	if uc.Level() >= u.Level() {
		return undecided
	}
	r := u.Level() - uc.Level()
	if r < votingRound {
		return undecided
	}
	if cachedResult, ok := uv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return cachedResult
	}

	defer func() {
		uv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	}()

	if r == votingRound {
		return uv.initialVote(uc, u)
	}

	commonVote := uv.lazyCommonVote(uc, u.Level()-1, uv.dag)
	var lastVote *vote
	voteUsingPrimeAncestors(uc, u, uv.dag, func(uc, uPrA gomel.Unit) (vote, bool) {
		result := uv.vote(uc, uPrA)
		if result == undecided {
			result = commonVote()
		}
		if lastVote != nil {
			if *lastVote != result {
				*lastVote = undecided
				return result, true
			}
		} else if result != undecided {
			lastVote = &result
		}
		return result, false

	})
	return *lastVote
}

func (uv *unanimousVoter) lazyCommonVote(uc gomel.Unit, round int, dag gomel.Dag) func() vote {
	initialized := false
	var commonVoteValue vote
	return func() vote {
		if !initialized {
			commonVoteValue = uv.commonVote(uc, round, dag)
			initialized = true
		}
		return commonVoteValue
	}
}

func (uv *unanimousVoter) initialVote(uc, u gomel.Unit) vote {
	if uc.Below(u) {
		return popular
	}
	return unpopular
}

// Deterministic function of a unit and level
// It is implemented as level-th bit of unit hash
// return 1 or 0
func simpleCoin(u gomel.Unit, level int) bool {
	index := level % (8 * len(u.Hash()))
	byteIndex, bitIndex := index/8, index%8
	return u.Hash()[byteIndex]&(1<<uint(bitIndex)) > 0
}

// Toss a coin using a given RandomSource.
// With low probability the toss may fail -- typically because of adversarial behavior of some process(es).
// uc - the unit whose popularity decision is being considered by tossing a coin
//      this param is used only in case when the simpleCoin is used, otherwise
//      the result of coin toss is meant to be a function of round only
// round - round for which we are tossing the coin
// returns: false or true -- a (pseudo)random bit, impossible to predict before level (round + 1) was reached
func coinToss(rs gomel.RandomSource, uc gomel.Unit, round int, dag gomel.Dag) bool {
	randomBytes := rs.RandomBytes(uc.Creator(), round+1)
	if randomBytes == nil {
		if simpleCoin(uc, round) {
			return true
		}
		return false
	}
	return randomBytes[0]&1 == 0
}

func (uv *unanimousVoter) commonVote(uc gomel.Unit, round int, dag gomel.Dag) vote {
	if round <= uc.Level() {
		return undecided
	}
	if round-uc.Level() <= votingRound {
		// "Default vote is asked on too low unit level."
		return undecided
	}
	if round <= deterministicPrefix {
		if round == 3 {
			return unpopular
		}
		return popular
	}
	if coinToss(uv.rs, uc, round, dag) {
		return popular
	}

	return unpopular
}
