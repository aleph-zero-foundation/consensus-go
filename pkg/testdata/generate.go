package main

import (
	"bufio"
	"math/rand"
	"os"
	"sync"
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

func writeToFile(filename string, dag gomel.Dag) error {
	file, err := os.Create(filename)
	defer file.Close()
	if err != nil {
		return err
	}
	out := bufio.NewWriter(file)
	tests.WriteDag(out, dag)
	out.Flush()
	return nil
}

// CreateRandomNonForkingUsingCreating creates a random test dag when given
// nProcesses - number of processes
// maxParents - maximal number of unit parents (valid for non-dealing units)
// nUnits     - number of units to include in the dag
func CreateRandomNonForkingUsingCreating(nProcesses, maxParents, nUnits int) gomel.Dag {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	p := growing.NewDag(&gomel.DagConfig{Keys: make([]gomel.PublicKey, nProcesses)})
	rs := tests.NewTestRandomSource(p)
	created := 0
	pus := make([]gomel.Preunit, nProcesses)
	for created < nUnits {
		pid := r.Intn(nProcesses)
		if pus[pid] != nil {
			var wg sync.WaitGroup
			wg.Add(1)
			p.AddUnit(pus[pid], rs, func(_ gomel.Preunit, _ gomel.Unit, _ error) {
				wg.Done()
			})
			wg.Wait()
			created++
			pus[pid] = nil
		} else {
			pu, err := creating.NewUnit(p, pid, maxParents, []byte{}, rs, true)
			if err != nil {
				continue
			}
			pus[pid] = pu
		}
	}
	return p
}

// Use this to generate more test files
func main() {
	writeToFile("dag.out", CreateRandomNonForkingUsingCreating(10, 2, 100))
}
