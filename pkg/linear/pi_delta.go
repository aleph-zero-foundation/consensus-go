package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type piDelta struct {
	dag           gomel.Dag
	piDeltaRound  uint64
	initialVoting voter
	coin          coinToss
	deltaMemo     map[[2]gomel.Hash]vote
	piMemo        map[[2]gomel.Hash]vote
}

func newPiDelta(dag gomel.Dag, piDeltaRound uint64, initialVoting voter, coin coinToss) *piDelta {
	return &piDelta{
		dag:           dag,
		piDeltaRound:  piDeltaRound,
		initialVoting: initialVoting,
		coin:          coin,
		deltaMemo:     make(map[[2]gomel.Hash]vote),
		piMemo:        make(map[[2]gomel.Hash]vote),
	}
}

func (pd *piDelta) decide(uc, u gomel.Unit) vote {
	return pd.computeDelta(uc, u)
}

// Computes the value of Delta from the paper
func (pd *piDelta) computeDelta(uc gomel.Unit, u gomel.Unit) vote {
	r := u.Level() - (uc.Level() + int(pd.piDeltaRound))
	if r%2 == 0 {
		// Delta used on an even level
		return undecided
	}
	if result, ok := pd.deltaMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return result
	}

	piValuesBelow := voteUsingPrimeAncestors(uc, u, pd.dag, pd.computePi)
	result := superMajority(pd.dag, piValuesBelow)
	pd.deltaMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	return result
}

// Computes the value of Pi from the paper
func (pd *piDelta) computePi(uc gomel.Unit, u gomel.Unit) vote {
	r := u.Level() - (uc.Level() + int(pd.piDeltaRound))
	if r < 0 {
		// PI-DELTA protocol used on a too low level
		return undecided
	}
	if result, ok := pd.piMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return result
	}

	voting := pd.computePi
	if r == 0 {
		voting = pd.initialVoting.vote
	}
	votesLevelBelow := voteUsingPrimeAncestors(uc, u, pd.dag, voting)
	var result vote
	if r%2 == 1 {
		result = existsTC(votesLevelBelow, uc, u, pd.coin)
	} else {
		result = superMajority(pd.dag, votesLevelBelow)
	}
	pd.piMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	return result
}

// Computes the exists function from the whitepaper, including the coin toss if necessary.
func existsTC(votes votingResult, uc gomel.Unit, u gomel.Unit, coin coinToss) vote {
	if votes.popular > 0 {
		return popular
	}
	if votes.unpopular > 0 {
		return unpopular
	}
	if coin(uc, u) {
		return popular
	}
	return unpopular
}
