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

const firstVotingRound = 1

type votingResult struct {
	popular   uint16
	unpopular uint16
}

type unanimousVoter struct {
	dag                           gomel.Dag
	rs                            gomel.RandomSource
	uc                            gomel.Unit
	firstRoundZeroForCommonVote   int
	commonVoteDeterministicPrefix int
	votingMemo                    map[gomel.Hash]vote
}

func newUnanimousVoter(uc gomel.Unit, dag gomel.Dag, rs gomel.RandomSource, commonVoteDeterministicPrefix int, firstRoundZeroForCommonVote int) *unanimousVoter {
	return &unanimousVoter{
		dag:                           dag,
		rs:                            rs,
		uc:                            uc,
		votingMemo:                    make(map[gomel.Hash]vote),
		commonVoteDeterministicPrefix: commonVoteDeterministicPrefix,
		firstRoundZeroForCommonVote:   firstRoundZeroForCommonVote,
	}
}

func (uv *unanimousVoter) VoteUsing(u gomel.Unit) (result vote) {
	r := u.Level() - uv.uc.Level()
	if r < firstVotingRound {
		return undecided
	}
	if cachedResult, ok := uv.votingMemo[*u.Hash()]; ok {
		return cachedResult
	}

	defer func() {
		uv.votingMemo[*u.Hash()] = result
	}()

	if r == firstVotingRound {
		return initialVote(uv.uc, u)
	}

	commonVote := uv.lazyCommonVote(u.Level() - 1)
	var lastVote *vote
	voteUsingPrimeAncestors(uv.uc, u, uv.dag, func(uc, uPrA gomel.Unit) (vote, bool) {
		result := uv.VoteUsing(uPrA)
		if result == undecided {
			result = commonVote()
		}
		if lastVote != nil {
			if *lastVote != result {
				*lastVote = undecided
				return result, true
			}
		} else {
			lastVote = &result
		}
		return result, false

	})
	if lastVote == nil {
		return undecided
	}
	return *lastVote
}

func (uv *unanimousVoter) lazyCommonVote(level int) func() vote {
	initialized := false
	var commonVoteValue vote
	return func() vote {
		if !initialized {
			commonVoteValue = uv.CommonVote(level)
			initialized = true
		}
		return commonVoteValue
	}
}

func initialVote(uc, u gomel.Unit) vote {
	if gomel.Above(u, uc) {
		return popular
	}
	return unpopular
}

// Toss a coin using a given RandomSource.
// uc - the unit whose popularity decision is being considered by tossing a coin
// level - level for which we are tossing the coin
// returns: false or true -- a (pseudo)random bit, impossible to predict before level was reached
func coinToss(uc gomel.Unit, level int, rs gomel.RandomSource) bool {
	return rs.RandomBytes(uc.Creator(), level)[0]&1 == 0
}

func (uv *unanimousVoter) CommonVote(level int) vote {
	round := level - uv.uc.Level()
	if round <= firstVotingRound {
		// "Default vote is asked on too low unit level."
		return undecided
	}
	if round <= uv.commonVoteDeterministicPrefix {
		if round == uv.firstRoundZeroForCommonVote {
			return unpopular
		}
		return popular
	}
	if coinToss(uv.uc, level+1, uv.rs) {
		return popular
	}

	return unpopular
}

// Checks if votes for popular or unpopular make a quorum.
// Returns the vote making a quorum or undecided if there is no quorum.
func superMajority(dag gomel.Dag, votes votingResult) vote {
	if dag.IsQuorum(votes.popular) {
		return popular
	}
	if dag.IsQuorum(votes.unpopular) {
		return unpopular
	}
	return undecided
}

func voteUsingPrimeAncestors(uc, u gomel.Unit, dag gomel.Dag, voter func(uc, u gomel.Unit) (vote vote, finish bool)) (votesLevelBelow votingResult) {
	for pid := range u.Parents() {
		floor := u.Floor(uint16(pid))
		votesOne := false
		votesZero := false
		finish := false
		for _, v := range floor {
			// find prime ancestor
			for predecessor := v; predecessor.Level() >= u.Level()-1; {
				v = predecessor
				predecessor = gomel.Predecessor(v)
				if predecessor == nil {
					break
				}
			}
			if v.Level() != u.Level()-1 {
				continue
			}

			// compute vote using prime ancestor
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
			return
		}
	}
	return votesLevelBelow
}
