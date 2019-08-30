package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

const (
	deterministicSuffix = 10
)

type superMajorityDecider struct {
	vote          *singleMindedVoter
	decidingRound uint64
}

func newSuperMajorityDecider(dag gomel.Dag, votingRound, decidingRound uint64, coinToss coinToss) *superMajorityDecider {
	commonVote := newCommonVote(votingRound, coinToss)
	vote := newSingleMindedVoter(dag, votingRound, commonVote)
	return &superMajorityDecider{
		vote:          vote,
		decidingRound: decidingRound,
	}
}

func (smd *superMajorityDecider) decide(uc, u gomel.Unit) vote {
	if uc.Level() >= u.Level() {
		return undecided
	}
	if uint64(u.Level()-uc.Level()) < smd.decidingRound {
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

type singleMindedVoter struct {
	dag         gomel.Dag
	votingRound uint64
	commonVote  commonVote
	votingMemo  map[[2]gomel.Hash]vote
}

func newSingleMindedVoter(dag gomel.Dag, votingRound uint64, commonVote commonVote) *singleMindedVoter {
	return &singleMindedVoter{
		dag:         dag,
		votingRound: votingRound,
		commonVote:  commonVote,
		votingMemo:  make(map[[2]gomel.Hash]vote),
	}
}

func (smv *singleMindedVoter) vote(uc, u gomel.Unit) (result vote) {
	if uc.Level() >= u.Level() {
		return undecided
	}
	r := uint64(u.Level() - uc.Level())
	if r < smv.votingRound {
		return undecided
	}
	if cachedResult, ok := smv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return cachedResult
	}

	defer func() {
		smv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	}()

	if r == smv.votingRound {
		return smv.initialVote(uc, u)
	}

	commonVote := smv.lazyCommonVote(uc, u)
	voter := func(uc, uPrA gomel.Unit) (vote, bool) {
		result := smv.vote(uc, uPrA)
		if result == undecided {
			result = commonVote()
		}
		return result, false
	}
	votesLevelBelow := voteUsingPrimeAncestors(uc, u, smv.dag, voter)
	return singleMinded(votesLevelBelow)
}

func (smv *singleMindedVoter) lazyCommonVote(uc, u gomel.Unit) func() vote {
	initialized := false
	var commonVoteValue vote
	return func() vote {
		if !initialized {
			commonVoteValue = smv.commonVote(uc, u)
			initialized = true
		}
		return commonVoteValue
	}
}

func (smv *singleMindedVoter) initialVote(uc, u gomel.Unit) vote {
	if uc.Below(u) {
		return popular
	}
	return unpopular
}

func newCommonVote(initialVotingRound uint64, coinToss coinToss) commonVote {
	return func(uc, u gomel.Unit) vote {
		if u.Level() <= uc.Level() {
			return undecided
		}
		r := uint64(u.Level() - uc.Level() - 1)
		if r <= initialVotingRound {
			// "Default vote is asked on too low unit level."
			return undecided
		}
		r = r - initialVotingRound
		if r <= deterministicSuffix {
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
