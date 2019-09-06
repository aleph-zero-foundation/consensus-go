package tests

import (
	"math/rand"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// CreateRandomNonForking creates a random test dag when given
//  nProcesses - number of processes
//  minParents - minimal number of unit's parents (valid for non-dealing units)
//  maxParents - maximal number of unit's parents (valid for non-dealing units)
//  nUnits     - number of units to include in the dag
func CreateRandomNonForking(nProcesses, minParents, maxParents, nUnits int) gomel.Dag {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	dag := newDag(gomel.DagConfig{Keys: make([]gomel.PublicKey, nProcesses)})
	rs := NewTestRandomSource()
	rs.Init(dag)
	created := 0
	for created < nUnits {
		pid := uint16(r.Intn(nProcesses))
		if dag.maximalHeight[pid] == -1 {
			pu := NewPreunit(pid, []*gomel.Hash{}, []byte{}, nil)
			dag.AddUnit(pu, rs, func(_ gomel.Preunit, _ gomel.Unit, _ error) {})
			created++
		} else {
			h := dag.maximalHeight[pid]
			parents := []*gomel.Hash{dag.unitsByHeight[h].Get(pid)[0].Hash()}
			nParents := minParents + r.Intn(maxParents-minParents+1)
			for _, parentID := range r.Perm(nProcesses) {
				if len(parents) == nParents {
					break
				}
				if parentID == int(pid) {
					continue
				}
				if dag.maximalHeight[parentID] != -1 {
					parents = append(parents, dag.MaximalUnitsPerProcess().Get(uint16(parentID))[0].Hash())
				}
				pu := NewPreunit(pid, parents, []byte{}, nil)
				if !checkExpandPrimes(dag, pu) {
					break
				}
			}
			if len(parents) >= minParents {
				pu := NewPreunit(pid, parents, []byte{}, nil)
				if checkExpandPrimes(dag, pu) {
					dag.AddUnit(pu, rs, func(_ gomel.Preunit, _ gomel.Unit, _ error) {})
					created++
				}
			}
		}
	}
	return dag
}
