package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type standardDecider struct {
	vote *standardVoter
}

func newStandardDecider(vote *standardVoter) *standardDecider {
	return &standardDecider{
		vote: vote,
	}
}

func (nv *standardDecider) decide(uc, u gomel.Unit) vote {
	if uc.Level() > u.Level() {
		return undecided
	}
	r := uint64(u.Level() - uc.Level())
	if r <= nv.vote.votingRound {
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
	defaultVote   defaultVote
	votingMemo    map[[2]gomel.Hash]vote
}

func newStandardVoter(dag gomel.Dag, votingRound uint64, initialVoting voter, defaultVote defaultVote) *standardVoter {
	return &standardVoter{
		dag:           dag,
		votingRound:   votingRound,
		initialVoting: initialVoting,
		defaultVote:   defaultVote,
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
