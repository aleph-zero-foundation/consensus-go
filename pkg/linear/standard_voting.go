package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type standardDecider struct {
	vote          *standardVoter
	decidingRound uint64
}

func newStandardDecider(vote *standardVoter, decidingRound uint64) *standardDecider {
	return &standardDecider{
		vote:          vote,
		decidingRound: decidingRound,
	}
}

func (sd *standardDecider) decide(uc, u gomel.Unit) vote {
	if uc.Level() > u.Level() {
		return undecided
	}
	if uint64(u.Level()-uc.Level()) < sd.decidingRound {
		return undecided
	}
	commonVote := sd.vote.commonVote(uc, u)
	result := undecided
	voter := func(uc, uPrA gomel.Unit) (vote, bool) {
		vote := sd.vote.vote(uc, uPrA)
		if vote != undecided && vote == commonVote {
			result = vote
			return vote, true
		}
		return vote, false
	}
	voteUsingPrimeAncestors(uc, u, sd.vote.dag, voter)
	return result
}

type standardVoter struct {
	dag           gomel.Dag
	votingRound   uint64
	initialVoting voter
	coinToss      coinToss
	votingMemo    map[[2]gomel.Hash]vote
}

func newStandardVoter(dag gomel.Dag, votingRound uint64, initialVoting voter, coinToss coinToss) *standardVoter {
	return &standardVoter{
		dag:           dag,
		votingRound:   votingRound,
		initialVoting: initialVoting,
		coinToss:      coinToss,
		votingMemo:    make(map[[2]gomel.Hash]vote),
	}
}

func (sv *standardVoter) vote(uc, u gomel.Unit) (result vote) {
	if uc.Level() > u.Level() {
		return undecided
	}
	r := uint64(u.Level() - uc.Level())
	if r < sv.votingRound {
		return undecided
	}
	if cachedResult, ok := sv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return cachedResult
	}

	defer func() {
		sv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	}()

	if r == sv.votingRound {
		return sv.initialVoting.vote(uc, u)
	}

	initialized := false
	var commonVoteValue vote
	commonVote := func() vote {
		if !initialized {
			commonVoteValue = sv.commonVote(uc, u)
			initialized = true
		}
		return commonVoteValue
	}
	voter := func(uc, uPrA gomel.Unit) (vote, bool) {
		result := sv.vote(uc, uPrA)
		if result == undecided {
			result = commonVote()
		}
		return result, false
	}
	votesLevelBelow := voteUsingPrimeAncestors(uc, u, sv.dag, voter)
	return superMajority(sv.dag, votesLevelBelow)
}

func (sv *standardVoter) commonVote(uc, u gomel.Unit) vote {
	if u.Level() <= uc.Level() {
		return undecided
	}
	r := uint64(u.Level() - uc.Level())
	if r <= sv.votingRound {
		// "Default vote is asked on too low unit level."
		return undecided
	}
	r = r - sv.votingRound
	if r <= 2 {
		if r == 1 {
			return unpopular
		}
		return popular
	}
	if sv.coinToss(uc, u) {
		return popular
	}
	return unpopular
}

type simpleInitialVoter struct {
}

func newSimpleInitialVoter() simpleInitialVoter {
	return simpleInitialVoter{}
}

func (simpleInitialVoter) vote(uc, u gomel.Unit) vote {
	if uc.Below(u) {
		return popular
	}
	return unpopular
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
