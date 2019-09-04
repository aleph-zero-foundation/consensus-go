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

const (
	firstVotingRound              = 1
	commonVoteDeterministicPrefix = 10
)

type votingResult struct {
	popular   uint64
	unpopular uint64
}

type unanimousVoter struct {
	dag        gomel.Dag
	rs         gomel.RandomSource
	votingMemo map[[2]gomel.Hash]vote
}

func newUnanimousVoter(dag gomel.Dag, rs gomel.RandomSource) *unanimousVoter {
	return &unanimousVoter{
		dag:        dag,
		rs:         rs,
		votingMemo: make(map[[2]gomel.Hash]vote),
	}
}

func (uv *unanimousVoter) vote(uc, u gomel.Unit) (result vote) {
	if uc.Level() >= u.Level() {
		return undecided
	}
	r := u.Level() - uc.Level()
	if r < firstVotingRound {
		return undecided
	}
	if cachedResult, ok := uv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return cachedResult
	}

	defer func() {
		uv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	}()

	if r == firstVotingRound {
		return uv.initialVote(uc, u)
	}

	commonVote := uv.lazyCommonVote(uc, u.Level()-1)
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

func (uv *unanimousVoter) lazyCommonVote(uc gomel.Unit, round int) func() vote {
	initialized := false
	var commonVoteValue vote
	return func() vote {
		if !initialized {
			commonVoteValue = uv.commonVote(uc, round)
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
func coinToss(uc gomel.Unit, round int, rs gomel.RandomSource) bool {
	randomBytes := rs.RandomBytes(uc.Creator(), round)
	if randomBytes == nil {
		if simpleCoin(uc, round) {
			return true
		}
		return false
	}
	return randomBytes[0]&1 == 0
}

func (uv *unanimousVoter) commonVote(uc gomel.Unit, round int) vote {
	if round <= uc.Level() {
		return undecided
	}
	if round-uc.Level() <= firstVotingRound {
		// "Default vote is asked on too low unit level."
		return undecided
	}
	if round <= commonVoteDeterministicPrefix {
		if round == 3 {
			return unpopular
		}
		return popular
	}
	if coinToss(uc, round+1, uv.rs) {
		return popular
	}

	return unpopular
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
			switch vote {
			case popular:
				votesOne = true
			case unpopular:
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
