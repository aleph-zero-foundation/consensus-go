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
