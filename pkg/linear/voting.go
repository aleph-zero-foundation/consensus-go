package linear

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	growing "gitlab.com/alephledger/consensus-go/pkg/growing"
)

// Checks whether v proves that uc is pupular on v's level.
// Which means that at least 2/3 * N processes created a unit w such that:
// (1) uc <= w <= v
// (2) level(w) <= level(v) - 2 or level(w) = level(v) - 1 and w is a prime unit
// returns 0 or 1
// It might be further optimized by using floors, but at this point gomel.Unit 
// doesn't define floors
func provesPopularity (p *growing.Poset, uc gomel.Unit, v gomel.Unit) int {
	//TODO: memo
	if uc.Level() >= v.Level() || !uc.Below(v) {
			return 0;
	}
	// simple BFS from v
	seenProcesses := make(map[int]int);
	seenUnits := make(map[*gomel.Unit]int);
	seenUnits[&v] = 1;
	queue := []*gomel.Unit{&v};
	for len(queue) > 0 {
		w := *queue[0]; 
		queue = queue[1:];
		if w.Level() <= v.Level() - 2 || (w.Level() == v.Level() - 1 && gomel.Prime(w)) {
			seenProcesses[w.Creator()] = 1;
			if p.IsQuorum(len(seenProcesses)) {
				return 1;
			}
		}
		for _, wParent := range(w.Parents()) {
			if _, exists := seenUnits[&wParent]; !exists && uc.Below(wParent) {
				queue = append(queue, &wParent);
				seenUnits[&wParent] = 1;
			}
		}
	}
	if p.IsQuorum(len(seenProcesses)) {
		return 1;
	} 
	return 0;
}

// Vote of u on popularity of uc as described in fast consenssus algorithm
// returns 0 or 1
func defaultVote(p *growing.Poset, u gomel.Unit, uc gomel.Unit) int {
	VOTING_LEVEL := 3; // TODO: Read this constant from config
	r := u.Level() - uc.Level() + VOTING_LEVEL;
	if r <= 0 {
		panic("Default vote is asked on too low unit level.");
	} 
	if r == 1 {
		return 1;
	}
	if r == 2 {
		return 0;
	}
	return simpleCoin(uc, u.Level());
}

// Deterministic function of a unit and level
// It is implemented as level-th bit of unit hash
// return 1 or 0
func simpleCoin(u gomel.Unit, level int) int {
	index := level % (8 * len(u.Hash()));
	byteIndex, bitIndex := index / 8, index % 8;
	if u.Hash()[byteIndex] & (1<<uint(bitIndex)) > 0 {
		return 1;
	} 
	return 0;
}

// Determine the vote of unit u on popularity of uc.
// If the first round of voting is at level L then:
// - at lvl L the vote is just whether u proves popularity of uc (i.e. whether uc <<< u)
// - at lvl (L+1) the vote is the supermajority of votes of prime ancestors (at level L)
// - at lvl (L+2) the vote is the supermajority of votes (replaced by default_vote if no supermajority) of prime ancestors (at level L+1)
// - etc.
// returns 0,1 or -1 (bot)
func computeVote(p *growing.Poset, u gomel.Unit, uc gomel.Unit) int {
	VOTING_LEVEL := 3 // TODO: Read this constant from config
	r := u.Level() - uc.Level() - VOTING_LEVEL
	if r < 0 {
		panic("Vote is asked on too low unit level.");
	}
	if r == 0 {
		return provesPopularity(p, u, uc); 
	} else {
		votesLevelBelow := []int{};
		primesLevelBelow := p.PrimeUnits(u.Level() - 1);
		primesLevelBelow.Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				voteV := computeVote(p, v, uc);
				if voteV == -1 {
					voteV = defaultVote(p, v, uc);
				}
				votesLevelBelow = append(votesLevelBelow, voteV);
			}
			return true;
		});
		return superMajority(p, votesLevelBelow);
	}
}

// Checks if votes for 0 or 1 makes quorum.
// returns 0 or 1 when there is supermajority 
// -1 (bot) otherwise 
func superMajority(p *growing.Poset, votes []int) int {
	cnt := make(map[int]int);
	for _, vote := range votes {
		cnt[vote]++;
	}
	if p.IsQuorum(cnt[0]) {
		return 0;
	}
	if p.IsQuorum(cnt[1]) {
		return 1;
	}
	return -1;
}

// Decides if uc is popular (i.e. it can be used as a timing unit)
// returns 0,1 (decision) or -1 (decision cannot be inferred yet)
func decideUnitIsPopular(p *growing.Poset, uc gomel.Unit) int {
	//TODO: memo
	VOTING_LEVEL, PI_DELTA_LEVEL := 3, 12; // TODO: Read this from config

	// At levels +2, +3,..., +(VOTING_LEVEL-1) it might be possible to prove that the consensus will be "1"
	// This is being tried in the loop below -- as Lemma 2.3.(1) in "Lewelewele" allows us to do:
	// -- whenever there is unit U at one of this levels that proves popularity of U_c, we can conclude the decision is "1"
	for level := uc.Level() + 2; level < uc.Level() + VOTING_LEVEL; level++ {
		decision := 0;
		p.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if provesPopularity(p, uc, v) == 1 {
					decision = 1;
					return false;
				}
			}
			return true;
		});
		if decision == 1 {
			return decision;
		}
	}
	
	// At level +VOTING_LEVEL+1, +VOTING_LEVEL+2, ..., +PI_DELTA_LEVEL-1 we use fast consensus algorithm
	for level := uc.Level() + VOTING_LEVEL; level < uc.Level() + PI_DELTA_LEVEL; level++ {
		decision := 0;
		p.PrimeUnits(level).Iterate(func(primes []gomel.Unit) bool {
			for _, v := range primes {
				if computeVote(p, v, uc) == 1 {
					decision = 1;
					return false;
				}
			}
			return true;
		});
		if decision == 1 {
			return decision;
		}
	}

	// at levels >= +PI_DELTA_LEVEL we use pi-delta consensus
	// TODO: implement PI_DELTA
	return -1;
}
