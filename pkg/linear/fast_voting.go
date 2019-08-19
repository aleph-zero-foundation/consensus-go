package linear

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type fastInitialVoting struct {
	dag       gomel.Dag
	proofMemo map[[2]gomel.Hash]bool
}

func newFastInitialVoting(dag gomel.Dag) *fastInitialVoting {
	return &fastInitialVoting{dag: dag, proofMemo: make(map[[2]gomel.Hash]bool)}
}

func (fv *fastInitialVoting) vote(uc, u gomel.Unit) vote {
	if fv.provesPopularity(uc, u) {
		return popular
	}
	return unpopular
}

type fastInitialDecider struct {
	fv          *fastInitialVoting
	votingLevel uint64
	fastDecider *standardDecider
}

func newFastDecider(
	dag gomel.Dag,
	fv *fastInitialVoting,
	votingLevel uint64,
	defaultVote defaultVote,
) *fastInitialDecider {
	fastVoter := newStandardVoter(dag, votingLevel, fv, defaultVote)
	fastDecider := newStandardDecider(dag, fastVoter)
	return &fastInitialDecider{
		fv:          fv,
		votingLevel: votingLevel,
		fastDecider: fastDecider,
	}
}

func (fv *fastInitialDecider) decide(uc, u gomel.Unit) vote {
	if u.Level() < uc.Level() {
		return undecided
	}
	r := uint64(u.Level() - uc.Level())
	if r >= fv.votingLevel {
		return fv.fastDecider.decide(uc, u)
	}
	if fv.fv.vote(uc, u) == popular {
		return popular
	}
	return undecided
}

// Checks whether v proves that uc is pupular on v's level.
// Which means that at least 2/3 * N processes created a unit w such that:
// (1) uc <= w <= v
// (2) level(w) <= level(v) - 2 or level(w) = level(v) - 1 and w is a prime unit
func (fv *fastInitialVoting) provesPopularity(uc gomel.Unit, v gomel.Unit) (isPopular bool) {
	if uc.Level() >= v.Level() || !uc.Below(v) {
		return false
	}
	if result, ok := fv.proofMemo[[2]gomel.Hash{*uc.Hash(), *v.Hash()}]; ok {
		return result
	}

	defer func() {
		fv.proofMemo[[2]gomel.Hash{*uc.Hash(), *v.Hash()}] = isPopular
	}()

	level := v.Level()
	nProcValid := 0
	nProcNotSeen := fv.dag.NProc()
	for _, myFloor := range v.Floor() {
		nProcNotSeen--
		// NOTE this loop can potentially visit same set of units on every spin
		for _, w := range myFloor {
			var reachedBottom error
			for w.Above(uc) && !((w.Level() <= level-2) || (w.Level() == level-1 && gomel.Prime(w))) {
				w, reachedBottom = gomel.Predecessor(w)
				if reachedBottom != nil {
					break
				}
			}
			if reachedBottom == nil && w.Above(uc) {
				nProcValid++
				if fv.dag.IsQuorum(nProcValid) {
					return true
				}
				break
			}
		}
		if !fv.dag.IsQuorum(nProcValid + nProcNotSeen) {
			return false
		}
	}
	return false
}
