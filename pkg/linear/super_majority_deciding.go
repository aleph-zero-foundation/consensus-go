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
	voter := func(uc, uPrA gomel.Unit) (vote, bool) {
		superMajorityVote := smd.decideUsingSuperMajority(uc, uPrA)
		if superMajorityVote != undecided && superMajorityVote == commonVote() {
			result = superMajorityVote
			return result, true
		}
		return superMajorityVote, false
	}
	voteUsingPrimeAncestors(uc, u, smd.vote.dag, voter)
	return result
}

func (smd *superMajorityDecider) decideUsingSuperMajority(uc, u gomel.Unit) vote {
	commonVote := smd.vote.lazyCommonVote(uc, u)
	voter := func(uc, uPrA gomel.Unit) (vote vote, finish bool) {
		result := smd.vote.vote(uc, uPrA)
		if result == undecided {
			result = commonVote()
		}
		return result, false
	}
	result := voteUsingPrimeAncestors(uc, u, smd.vote.dag, voter)
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
	voter := func(uc, uPrA gomel.Unit) (vote, bool) {
		result := uv.vote(uc, uPrA)
		if result == undecided {
			result = commonVote()
		}
		return result, false
	}
	votesLevelBelow := voteUsingPrimeAncestors(uc, u, uv.dag, voter)
	return unanimousVoting(votesLevelBelow)
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
		if !(votesOne || votesZero) {
			votesLevelBelow.undecided++
		}
		if finish {
			return false
		}
		return true
	})
	return votesLevelBelow
}
