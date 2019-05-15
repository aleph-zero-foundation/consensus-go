package tests

import (
	"math/rand"
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// CreateRandomNonForking creates a random test poset when given
// nProcesses - number of processes
// minParents - minimal number of unit parents (valid for non-dealing units)
// maxParents - maximal number of unit parents (valid for non-dealing units)
// nUnits     - number of units to include in the poset
func CreateRandomNonForking(nProcesses, minParents, maxParents, nUnits int) gomel.Poset {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	p := newPoset(gomel.PosetConfig{Keys: make([]gomel.PublicKey, nProcesses)})
	created := 0
	for created < nUnits {
		// TODO:
		// (1) check if the compliance rules are satisfied
		//		 (currently for this non-forking example expand prime rule might not be satisfied)
		// (2) add new parameter to the function containing
		//		 set of compliance rules which should be satisfied
		pid := r.Intn(nProcesses)
		if p.maximalHeight[pid] == -1 {
			pu := newPreunit(pid, []gomel.Hash{}, []gomel.Tx{})
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
				pu := newPreunit(pid, parents, []gomel.Tx{})
				p.AddUnit(pu, func(_ gomel.Preunit, _ gomel.Unit, _ error) {})
				created++
			}
		}
	}
	return p
}
