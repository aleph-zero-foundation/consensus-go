package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

const (
	decidingRound       = 3
	votingRound         = 1
	deterministicPrefix = 10
)

type deciderGovernor struct {
	smd *superMajorityDecider
}

func newDeciderGovernor(dag gomel.Dag) deciderGovernor {
	return deciderGovernor{smd: newSuperMajorityDecider(dag)}
}

func (dg deciderGovernor) initializeDecider(uc gomel.Unit, maxDagLevel int) (decider *superMajorityDecider, maxLevel int) {
	decider = dg.smd
	return decider, decider.getMaximalLevelForDecision(uc, maxDagLevel)
}

type superMajorityDecider struct {
	vote *unanimousVoter
}

func newSuperMajorityDecider(dag gomel.Dag) *superMajorityDecider {
	vote := newUnanimousVoter(dag)
	return &superMajorityDecider{
		vote: vote,
	}
}

func (smd *superMajorityDecider) getMaxDecisionLevel(uc gomel.Unit, dagMaxLevelReached int) (maxAvailableLevel int) {
	if (dagMaxLevelReached - uc.Level()) < deterministicPrefix {
		return dagMaxLevelReached
	}
	return (dagMaxLevelReached - 2)
}

func (smd *superMajorityDecider) decide(uc, u gomel.Unit) vote {
	if uc.Level() >= u.Level() {
		return undecided
	}
	if u.Level()-uc.Level() < decidingRound {
		return undecided
	}
	commonVote := smd.vote.lazyCommonVote(uc, u.Level(), smd.vote.dag)
	result := smd.decideUsingSuperMajorityOfVotes(uc, u)
	if result != undecided && result == commonVote() {
		return result
	}
	return undecided
}

func (smd *superMajorityDecider) decideUsingSuperMajorityOfVotes(uc, u gomel.Unit) vote {
	commonVote := smd.vote.lazyCommonVote(uc, u.Level()-1, smd.vote.dag)
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

func (smd *superMajorityDecider) getMaximalLevelForDecision(uc gomel.Unit, dagMaxLevel int) int {
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

func coinToss(rs gomel.RandomSource, uc gomel.Unit, round int, dag gomel.Dag) bool {
	randomBytes := rs.RandomBytes(uc.Creator(), round+1)
	if randomBytes == nil {
		if simpleCoin(uc, round+1) {
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
