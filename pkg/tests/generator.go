package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"math/rand"
	"time"
)

// CreateRandomNonForking creates a random test poset when given
// nProcesses - number of processes
// minParents - minimal number of unit parents (valid for non-dealing units)
// maxParents - maximal number of unit parents (valid for non-dealing units)
// nUnits     - number of units to include in the poset
func CreateRandomNonForking(nProcesses, minParents, maxParents, nUnits int) gomel.Poset {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	p := newPoset(nProcesses)
	created := 0
	for created < nUnits {
		pid := r.Intn(nProcesses)
		if p.maximalHeight[pid] == -1 {
			pu := newPreunit(pid, []gomel.Hash{})
			p.AddUnit(pu, func(_ gomel.Preunit, _ gomel.Unit, _ error) {})
			created++
		} else {
			h := p.maximalHeight[pid]
			parents := []gomel.Hash{*p.unitsByHeight[h].Get(pid)[0].Hash()}
			nParents := minParents + r.Intn(maxParents-minParents+1)
			for _, parentID := range r.Perm(nProcesses) {
				if len(parents) == nParents {
					break
				}
				if parentID == pid {
					continue
				}
				if p.maximalHeight[parentID] != -1 {
					parents = append(parents, *p.MaximalUnitsPerProcess().Get(parentID)[0].Hash())
				}
			}
			if len(parents) >= minParents {
				pu := newPreunit(pid, parents)
				p.AddUnit(pu, func(_ gomel.Preunit, _ gomel.Unit, _ error) {})
				created++
			}
		}
	}
	return p
}
