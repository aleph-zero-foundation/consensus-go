package linear

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type vote int

const (
	popular vote = iota
	unpopular
	undecided
	votingLevel  = 3  // todo: pull from config
	piDeltaLevel = 12 // todo: pull from config
)

// Checks whether v proves that uc is pupular on v's level.
// Which means that at least 2/3 * N processes created a unit w such that:
// (1) uc <= w <= v
// (2) level(w) <= level(v) - 2 or level(w) = level(v) - 1 and w is a prime unit
// It might be further optimized by using floors, but at this point gomel.Unit
// doesn't define floors
func provesPopularity(p gomel.Poset, uc gomel.Unit, v gomel.Unit) bool {
	//TODO: memo
	if uc.Level() >= v.Level() || !uc.Below(v) {
		return false
	}
	// simple BFS from v
	seenProcesses := make(map[int]bool)
	seenUnits := make(map[gomel.Hash]bool)
	seenUnits[*v.Hash()] = true
	queue := []gomel.Unit{v}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if w.Level() <= v.Level()-2 || (w.Level() == v.Level()-1 && gomel.Prime(w)) {
			seenProcesses[w.Creator()] = true
			if p.IsQuorum(len(seenProcesses)) {
				return true
			}
		}
		for _, wParent := range w.Parents() {
			if _, exists := seenUnits[*wParent.Hash()]; !exists && uc.Below(wParent) {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	if p.IsQuorum(len(seenProcesses)) {
		return true
	}
	return false
}

// Vote of u on popularity of uc as described in fast consenssus algorithm
func defaultVote(u gomel.Unit, uc gomel.Unit) vote {
	r := u.Level() - uc.Level() + votingLevel
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
func computeVote(p gomel.Poset, u gomel.Unit, uc gomel.Unit) vote {
	r := u.Level() - uc.Level() - votingLevel
	if r < 0 {
		//"Vote is asked on too low unit level."
		return undecided
	}
	if r == 0 {
		if provesPopularity(p, uc, u) {
			return popular
		}
		return unpopular
	}
	votesLevelBelow := []vote{}
	primesLevelBelow := p.PrimeUnits(u.Level() - 1)
	primesLevelBelow.Iterate(func(primes []gomel.Unit) bool {
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			voteV := computeVote(p, v, uc)
			if voteV == undecided {
				voteV = defaultVote(v, uc)
			}
			votesLevelBelow = append(votesLevelBelow, voteV)
		}
		return true
	})
	return superMajority(p, votesLevelBelow)

}

// Checks if votes for popular or unpopular makes a quorum.
// returns the vote making a quorum or undecided if there is no quorum
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

func coinToss(p gomel.Poset, uc gomel.Unit, u gomel.Unit) int {
	// TODO: implement using threshold coin
	return 0
}

// Computes the exists function from the whitepaper, including the coin toss if necessary.
func existsTC(p gomel.Poset, votes []vote, uc gomel.Unit, u gomel.Unit) vote {
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

	if coinToss(p, uc, u) == 1 {
		return popular
	}
	return unpopular
}

// Computes the value of Pi from the paper
func computePi(p gomel.Poset, uc gomel.Unit, u gomel.Unit) vote {
	r := u.Level() - (uc.Level() + piDeltaLevel)
	if r < 0 {
		// PI-DELTA protocol used on a too low level
		return undecided
	}
	votesLevelBelow := []vote{}
	primesLevelBelow := p.PrimeUnits(u.Level() - 1)
	primesLevelBelow.Iterate(func(primes []gomel.Unit) bool {
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			if r == 0 {
				voteV := computeVote(p, v, uc)
				if voteV == undecided {
					voteV = defaultVote(v, uc)
				}
				votesLevelBelow = append(votesLevelBelow, voteV)
			} else {
				votesLevelBelow = append(votesLevelBelow, computePi(p, uc, u))
			}
		}
		return true
	})
	if r%2 == 1 {
		return existsTC(p, votesLevelBelow, uc, u)
	}
	return superMajority(p, votesLevelBelow)
}

// Computes the value of Delta from the paper
func computeDelta(p gomel.Poset, uc gomel.Unit, u gomel.Unit) vote {
	r := u.Level() - (uc.Level() + piDeltaLevel)
	if r%2 == 0 {
		// Delta used on an even level
		return undecided
	}
	piValuesBelow := []vote{}
	primesLevelBelow := p.PrimeUnits(u.Level() - 1)
	primesLevelBelow.Iterate(func(primes []gomel.Unit) bool {
		for _, v := range primes {
			if !v.Below(u) {
				continue
			}
			piValuesBelow = append(piValuesBelow, computePi(p, uc, v))
		}
		return true
	})
	return superMajority(p, piValuesBelow)
}

// Decides if uc is popular (i.e. it can be used as a timing unit)
// Returns vote
func decideUnitIsPopular(p gomel.Poset, uc gomel.Unit) vote {
	//TODO: memo

	posetLevelReached := posetMaxLevel(p)
	// At levels +2, +3,..., +(votingLevel-1) it might be possible to prove that the consensus will be "1"
	// This is being tried in the loop below -- as Lemma 2.3.(1) in "Lewelewele" allows us to do:
	// -- whenever there is unit U at one of this levels that proves popularity of U_c, we can conclude the decision is "1"
	for level := uc.Level() + 2; level < uc.Level()+votingLevel && level <= posetLevelReached; level++ {
		decision := undecided
		p.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if provesPopularity(p, uc, v) {
					decision = popular
					return false
				}
			}
			return true
		})
		if decision == popular {
			return decision
		}
	}

	// At level +votingLevel+1, +votingLevel+2, ..., +piDeltaLevel-1 we use fast consensus algorithm
	for level := uc.Level() + votingLevel + 1; level < uc.Level()+piDeltaLevel && level <= posetLevelReached; level++ {
		decision := undecided
		p.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if computeVote(p, v, uc) == defaultVote(v, uc) {
					decision = defaultVote(v, uc)
					return false
				}
			}
			return true
		})
		if decision != undecided {
			return decision
		}
	}

	// at levels >= +piDeltaLevel we use pi-delta consensus
	// The decisions (delta) are made at levels +piDeltaLevel+1, +piDeltaLevel+3, etc
	// whereas decisions in the paper are made at levels: +2, +4, etc
	// Therefore the round type r := v.Level() - uc.Level() - piDeltaLevel
	// used in computePi and computeDelta and R_uc(v) defined in the paper have opposite parity
	for level := uc.Level() + piDeltaLevel + 1; level <= posetLevelReached; level += 2 {
		decision := undecided
		p.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if voteV := computeDelta(p, uc, v); voteV != undecided {
					decision = voteV
					return false
				}
			}
			return true
		})
		if decision != undecided {
			return decision
		}
	}

	return undecided
}
