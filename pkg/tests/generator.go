package tests

import (
	"math/rand"
	"time"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// CreateRandomNonForking creates a random test dag when given
//  nProcesses - number of processes
//  nUnits     - number of units to include in the dag
func CreateRandomNonForking(nProcesses, nUnits int) gomel.Dag {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	dag := newDag(gomel.DagConfig{Keys: make([]gomel.PublicKey, nProcesses)})
	created := 0
	for created < nUnits {
		pid := uint16(r.Intn(nProcesses))
		if dag.maximalHeight[pid] == -1 {
			pu := NewPreunit(pid, make([]*gomel.Hash, nProcesses), []byte{}, nil)
			_, err := gomel.AddUnit(dag, pu)
			if err == nil {
				created++
			}
		} else {
			parents := make([]*gomel.Hash, dag.NProc())
			for i := uint16(0); i < dag.NProc(); i++ {
				h := dag.maximalHeight[i]
				if h == -1 {
					parents[i] = nil
				} else {
					parents[i] = dag.unitsByHeight[h].Get(i)[0].Hash()
				}
			}
			pu := NewPreunit(pid, parents, []byte{}, nil)
			_, err := gomel.AddUnit(dag, pu)
			if err == nil {
				created++
			}
		}
	}
	return dag
}
