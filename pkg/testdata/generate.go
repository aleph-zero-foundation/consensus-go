package main

import (
	"bufio"
	"os"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
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
func CreateRandomNonForkingUsingCreating(nProcesses, maxParents uint16, nUnits int) gomel.Dag {
	//r := rand.New(rand.NewSource(time.Now().UnixNano()))
	dag := dag.New(nProcesses)
	rs := tests.NewTestRandomSource()
	dag = rs.Bind(dag)
	created := 0
	pus := make([]gomel.Preunit, nProcesses)
	pid := -1
	for created < nUnits {
		pid = (pid + 1) % nProcesses
		if pus[pid] != nil {
			var wg sync.WaitGroup
			wg.Add(1)
			dag.AddUnit(pus[pid], func(_ gomel.Preunit, _ gomel.Unit, _ error) {
				wg.Done()
			})
			wg.Wait()
			created++
			pus[pid] = nil
		} else {
			pu, err := creating.NewUnit(dag, pid, maxParents, []byte{}, rs, true)
			if err != nil {
				continue
			}
			pus[pid] = pu
		}
	}
	return dag
}

// Use this to generate more test files
func main() {
	writeToFile("dag.out", CreateRandomNonForkingUsingCreating(4, 60))
}
