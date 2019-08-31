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
	vote *unanimousVoter
}

func newSuperMajorityDecider(dag gomel.Dag, coinToss coinToss) *superMajorityDecider {
	commonVote := newCommonVote(coinToss)
	vote := newUnanimousVoter(dag, commonVote)
	return &superMajorityDecider{
		vote: vote,
	}
}

func (smd *superMajorityDecider) decide(uc, u gomel.Unit) vote {
	if uc.Level() >= u.Level() {
		return undecided
	}
	if u.Level()-uc.Level() < decidingRound {
		return undecided
	}
	commonVote := smd.vote.lazyCommonVote(uc, u)
	result := undecided
	voteUsingPrimeAncestors(uc, u, smd.vote.dag, func(uc, uPrA gomel.Unit) (vote, bool) {
		superMajorityVote := smd.decideUsingSuperMajorityOfVotes(uc, uPrA)
		if superMajorityVote != undecided && superMajorityVote == commonVote() {
			result = superMajorityVote
			return result, true
		}
		return superMajorityVote, false
	})
	return result
}

func (smd *superMajorityDecider) decideUsingSuperMajorityOfVotes(uc, u gomel.Unit) vote {
	commonVote := smd.vote.lazyCommonVote(uc, u)
	var votingResult votingResult
	result := voteUsingPrimeAncestors(uc, u, smd.vote.dag, func(uc, uPrA gomel.Unit) (vote vote, finish bool) {
		result := smd.vote.vote(uc, uPrA)
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
			if superMajority(smd.vote.dag, votingResult) != undecided {
				return result, true
			}
		} else {
			// fast fail
			test := votingResult
			remaining := uint64(smd.vote.dag.NProc() - uPrA.Creator() - 1)
			test.popular += remaining
			test.unpopular += remaining
			if superMajority(smd.vote.dag, test) == undecided {
				return result, true
			}
		}

		return result, false
	})
	return superMajority(smd.vote.dag, result)
}

type unanimousVoter struct {
	dag        gomel.Dag
	commonVote commonVote
	votingMemo map[[2]gomel.Hash]vote
}

func newUnanimousVoter(dag gomel.Dag, commonVote commonVote) *unanimousVoter {
	return &unanimousVoter{
		dag:        dag,
		commonVote: commonVote,
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

	commonVote := uv.lazyCommonVote(uc, u)
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

func (uv *unanimousVoter) lazyCommonVote(uc, u gomel.Unit) func() vote {
	initialized := false
	var commonVoteValue vote
	return func() vote {
		if !initialized {
			commonVoteValue = uv.commonVote(uc, u)
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

func newCommonVote(coinToss coinToss) commonVote {
	return func(uc, u gomel.Unit) vote {
		if u.Level() <= uc.Level() {
			return undecided
		}
		r := u.Level() - uc.Level() - 1
		if r <= votingRound {
			// "Default vote is asked on too low unit level."
			return undecided
		}
		r = r - votingRound
		if r <= deterministicPrefix {
			if r%2 == 1 {
				return popular
			}
			return unpopular
		}
		if coinToss(uc, u) {
			return popular
		}
		return unpopular
	}
}
