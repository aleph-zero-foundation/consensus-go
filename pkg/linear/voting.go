package linear

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type vote int

const (
	popular vote = iota
	unpopular
	undecided
)

// Checks whether v proves that uc is pupular on v's level.
// Which means that at least 2/3 * N processes created a unit w such that:
// (1) uc <= w <= v
// (2) level(w) <= level(v) - 2 or level(w) = level(v) - 1 and w is a prime unit
func (o *ordering) provesPopularity(uc gomel.Unit, v gomel.Unit) bool {
	if uc.Level() >= v.Level() || !uc.Below(v) {
		return false
	}
	if result, ok := o.proofMemo[[2]gomel.Hash{*uc.Hash(), *v.Hash()}]; ok {
		return result
	}

	level := v.Level()
	nProcValid := 0
	nProcNotSeen := o.poset.NProc()
	for _, myFloor := range v.Floor() {
		nProcNotSeen--
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
				if o.poset.IsQuorum(nProcValid) {
					o.proofMemo[[2]gomel.Hash{*uc.Hash(), *v.Hash()}] = true
					return true
				}
				break
			}
		}
		if !o.poset.IsQuorum(nProcValid + nProcNotSeen) {
			o.proofMemo[[2]gomel.Hash{*uc.Hash(), *v.Hash()}] = false
			return false
		}
	}
	result := o.poset.IsQuorum(nProcValid)
	o.proofMemo[[2]gomel.Hash{*uc.Hash(), *v.Hash()}] = result
	return result
}

// Vote of u on popularity of uc as described in fast consensus algorithm
func (o *ordering) defaultVote(u gomel.Unit, uc gomel.Unit) vote {
	r := u.Level() - uc.Level() - o.votingLevel
	if r <= 0 {
		// "Default vote is asked on too low unit level."
		return undecided
	}
	if r == 1 {
		return popular
	}
	if r == 2 {
		return unpopular
	}
	coinToss := simpleCoin(uc, u.Level())
	if coinToss == 0 {
		return popular
	}
	return unpopular
}

// Deterministic function of a unit and level
// It is implemented as level-th bit of unit hash
// return 1 or 0
func simpleCoin(u gomel.Unit, level int) int {
	index := level % (8 * len(u.Hash()))
	byteIndex, bitIndex := index/8, index%8
	if u.Hash()[byteIndex]&(1<<uint(bitIndex)) > 0 {
		return 1
	}
	return 0
}

// Determine the vote of unit u on popularity of uc.
// If the first round of voting is at level L then:
// - at lvl L the vote is just whether u proves popularity of uc (i.e. whether uc <<< u)
// - at lvl (L+1) the vote is the supermajority of votes of prime ancestors (at level L)
// - at lvl (L+2) the vote is the supermajority of votes (replaced by default_vote if no supermajority) of prime ancestors (at level L+1)
// - etc.
func (o *ordering) computeVote(u gomel.Unit, uc gomel.Unit) vote {
	r := u.Level() - uc.Level() - o.votingLevel
	if r < 0 {
		//"Vote is asked on too low unit level."
		return undecided
	}

	if result, ok := o.voteMemo[[2]gomel.Hash{*u.Hash(), *uc.Hash()}]; ok {
		return result
	}

	if r == 0 {
		if o.provesPopularity(uc, u) {
			return popular
		}
		return unpopular
	}
	votesLevelBelow := []vote{}
	primesLevelBelow := o.poset.PrimeUnits(u.Level() - 1)
	primesLevelBelow.Iterate(func(primes []gomel.Unit) bool {
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			voteV := o.computeVote(v, uc)
			if voteV == undecided {
				voteV = o.defaultVote(v, uc)
			}
			votesLevelBelow = append(votesLevelBelow, voteV)
		}
		return true
	})
	result := superMajority(o.poset, votesLevelBelow)
	o.voteMemo[[2]gomel.Hash{*u.Hash(), *uc.Hash()}] = result
	return result
}

// Checks if votes for popular or unpopular make a quorum.
// Returns the vote making a quorum or undecided if there is no quorum.
func superMajority(p gomel.Poset, votes []vote) vote {
	cnt := make(map[vote]int)
	for _, vote := range votes {
		cnt[vote]++
	}
	if p.IsQuorum(cnt[popular]) {
		return popular
	}
	if p.IsQuorum(cnt[unpopular]) {
		return unpopular
	}
	return undecided
}

// coinToss at unit uTossing (necessarily at level >= ADD_SHARES + 1)
// With low probability the toss may fail -- typically because of adversarial behavior of some process(es).
// uc - the unit whose popularity decision is being considered by tossing a coin
//      this param is used only in case when the simpleCoin is used, otherwise
//      the result of coin toss is meant to be a function of uTossing.level() only
// uTossing - the unit that is cossing a toin
// returns: 0 or 1 -- a (pseudo)random bit, impossible to predict before (uTossing.level - 1) was reached

func (o *ordering) coinToss(uc gomel.Unit, uTossing gomel.Unit) int {
	level := uTossing.Level() - 1
	randomBytes := o.randomSource.RandomBytes(uTossing, uTossing.Level())
	if randomBytes == nil {
		return simpleCoin(uc, level)
	}
	return int(randomBytes[0] & 1)
}

// Computes the exists function from the whitepaper, including the coin toss if necessary.
func (o *ordering) existsTC(votes []vote, uc gomel.Unit, u gomel.Unit) vote {
	for _, voteConsidered := range votes {
		if voteConsidered == popular {
			return popular
		}
	}

	for _, voteConsidered := range votes {
		if voteConsidered == unpopular {
			return unpopular
		}
	}

	if o.coinToss(uc, u) == 1 {
		return popular
	}
	return unpopular
}

// Computes the value of Pi from the paper
func (o *ordering) computePi(uc gomel.Unit, u gomel.Unit) vote {
	r := u.Level() - (uc.Level() + o.piDeltaLevel)
	if r < 0 {
		// PI-DELTA protocol used on a too low level
		return undecided
	}
	if result, ok := o.piMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return result
	}

	votesLevelBelow := []vote{}
	primesLevelBelow := o.poset.PrimeUnits(u.Level() - 1)
	primesLevelBelow.Iterate(func(primes []gomel.Unit) bool {
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			if r == 0 {
				voteV := o.computeVote(v, uc)
				if voteV == undecided {
					voteV = o.defaultVote(v, uc)
				}
				votesLevelBelow = append(votesLevelBelow, voteV)
			} else {
				votesLevelBelow = append(votesLevelBelow, o.computePi(uc, u))
			}
		}
		return true
	})
	var result vote
	if r%2 == 1 {
		result = o.existsTC(votesLevelBelow, uc, u)
	} else {
		result = superMajority(o.poset, votesLevelBelow)
	}
	o.piMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	return result
}

// Computes the value of Delta from the paper
func (o *ordering) computeDelta(uc gomel.Unit, u gomel.Unit) vote {
	r := u.Level() - (uc.Level() + o.piDeltaLevel)
	if r%2 == 0 {
		// Delta used on an even level
		return undecided
	}
	if result, ok := o.deltaMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}]; ok {
		return result
	}

	piValuesBelow := []vote{}
	primesLevelBelow := o.poset.PrimeUnits(u.Level() - 1)
	primesLevelBelow.Iterate(func(primes []gomel.Unit) bool {
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			piValuesBelow = append(piValuesBelow, o.computePi(uc, v))
		}
		return true
	})
	result := superMajority(o.poset, piValuesBelow)
	o.deltaMemo[[2]gomel.Hash{*uc.Hash(), *u.Hash()}] = result
	return result
}

// Decides if uc is popular (i.e. it can be used as a timing unit)
// Returns vote
func (o *ordering) decideUnitIsPopular(uc gomel.Unit) vote {
	if result, ok := o.decisionMemo[*uc.Hash()]; ok {
		return result
	}

	posetLevelReached := posetMaxLevel(o.poset)
	// At levels +2, +3,..., +(votingLevel-1) it might be possible to prove that the consensus will be "1"
	// This is being tried in the loop below -- as Lemma 2.3.(1) in "Lewelewele" allows us to do:
	// -- whenever there is unit U at one of this levels that proves popularity of U_c, we can conclude the decision is "1"
	for level := uc.Level() + 2; level < uc.Level()+o.votingLevel && level <= posetLevelReached; level++ {
		decision := undecided
		o.poset.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if o.provesPopularity(uc, v) {
					decision = popular
					return false
				}
			}
			return true
		})
		if decision == popular {
			o.decisionMemo[*uc.Hash()] = decision
			return decision
		}
	}

	// At level +votingLevel+1, +votingLevel+2, ..., +piDeltaLevel-1 we use fast consensus algorithm
	for level := uc.Level() + o.votingLevel + 1; level < uc.Level()+o.piDeltaLevel && level <= posetLevelReached; level++ {
		decision := undecided
		o.poset.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if o.computeVote(v, uc) == o.defaultVote(v, uc) {
					decision = o.defaultVote(v, uc)
					return false
				}
			}
			return true
		})
		if decision != undecided {
			o.decisionMemo[*uc.Hash()] = decision
			return decision
		}
	}

	// at levels >= +piDeltaLevel we use pi-delta consensus
	// The decisions (delta) are made at levels +piDeltaLevel+1, +piDeltaLevel+3, etc
	// whereas decisions in the paper are made at levels: +2, +4, etc
	// Therefore the round type r := v.Level() - uc.Level() - piDeltaLevel
	// used in computePi and computeDelta and R_uc(v) defined in the paper have opposite parity
	for level := uc.Level() + o.piDeltaLevel + 1; level <= posetLevelReached; level += 2 {
		decision := undecided
		o.poset.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if voteV := o.computeDelta(uc, v); voteV != undecided {
					decision = voteV
					return false
				}
			}
			return true
		})
		if decision != undecided {
			o.decisionMemo[*uc.Hash()] = decision
			return decision
		}
	}

	return undecided
}
