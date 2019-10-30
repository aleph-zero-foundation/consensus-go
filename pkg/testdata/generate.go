package main

import (
	"bufio"
	"math/rand"
	"os"
	"time"

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
// nUnits     - number of units to include in the dag
// canSkipLevel - if the processes can skip some levels
func CreateRandomNonForkingUsingCreating(nProcesses uint16, nUnits int, canSkipLevel bool) gomel.Dag {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	dag := dag.New(nProcesses)
	rs := tests.NewTestRandomSource()
	dag = rs.Bind(dag)
	created := 0
	pus := make([]gomel.Preunit, nProcesses)
	for created < nUnits {
		pid := uint16(r.Intn(int(nProcesses)))
		if pus[pid] != nil {
			_, err := gomel.AddUnit(dag, pus[pid])
			if err == nil {
				created++
				pus[pid] = nil
			}
		} else {
			pu, _, err := creating.NewUnit(dag, pid, []byte{}, rs, canSkipLevel)
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
	writeToFile("dag.out", CreateRandomNonForkingUsingCreating(10, 100, true))
}
