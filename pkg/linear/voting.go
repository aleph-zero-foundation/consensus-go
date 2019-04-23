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
