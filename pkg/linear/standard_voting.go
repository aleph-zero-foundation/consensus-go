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

func (nv *standardDecider) decide(uc, u gomel.Unit) vote {
	if uc.Level() > u.Level() {
		return undecided
	}
	r := uint64(u.Level() - uc.Level())
	if r < nv.decidingRound {
		return undecided
	}
	decision := nv.vote.vote(uc, u)
	if decision != undecided && decision == nv.vote.defaultVote(uc, u) {
		return decision
	}
	return undecided
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

func (rv *standardVoter) vote(uc, u gomel.Unit) (result vote) {
	if uc.Level() > u.Level() {
		return undecided
	}
	r := uint64(u.Level() - uc.Level())
	if r < rv.votingRound {
		return undecided
	}
	if cachedResult, ok := rv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return cachedResult
	}

	defer func() {
		rv.votingMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	}()

	if r == rv.votingRound {
		if rv.initialVoting.vote(uc, u) == popular {
			return popular
		}
		return unpopular
	}

	voter := func(uc, u gomel.Unit) vote {
		result := rv.vote(uc, u)
		if result == undecided {
			result = rv.defaultVote(uc, u)
		}
		return result
	}
	votesLevelBelow := voteUsingPrimeAncestors(uc, u, rv.dag, voter)
	return superMajority(rv.dag, votesLevelBelow)
}

func (rv *standardVoter) defaultVote(uc, u gomel.Unit) (result vote) {
	r := u.Level() - uc.Level()
	if r <= 0 {
		// "Default vote is asked on too low unit level."
		return undecided
	}
	if r <= 3 {
		return popular
	}
	if r == 4 {
		return unpopular
	}
	if rv.coinToss(uc, u) {
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

func voteUsingPrimeAncestors(uc, u gomel.Unit, dag gomel.Dag, voter func(uc, u gomel.Unit) vote) (votesLevelBelow votingResult) {
	dag.PrimeUnits(u.Level() - 1).Iterate(func(primes []gomel.Unit) bool {
		votesOne := false
		votesZero := false
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			if vote := voter(uc, v); vote == popular {
				votesOne = true
			} else if vote == unpopular {
				votesZero = true
			}
			if votesOne && votesZero {
				break
			}
		}
		if votesOne {
			votesLevelBelow.popular++
		}
		if votesZero {
			votesLevelBelow.unpopular++
		}
		return true
	})
	return votesLevelBelow
}
