package tests

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/transactions"
)

// ReadDag reads dag from the given reader and creates it using given dag factory
func ReadDag(reader io.Reader, df gomel.DagFactory) (gomel.Dag, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Scan()
	text := scanner.Text()
	var n int

	_, err := fmt.Sscanf(text, "%d", &n)
	if err != nil {
		return nil, err
	}

	dag := df.CreateDag(gomel.DagConfig{Keys: make([]gomel.PublicKey, n)})
	rs := NewTestRandomSource()
	rs.Init(dag)
	preunitHashes := make(map[[3]int]*gomel.Hash)

	var txID uint32

	for scanner.Scan() {
		text := scanner.Text()
		var puCreator, puHeight, puVersion int
		parents := []*gomel.Hash{}
		for i, t := range strings.Split(text, " ") {
			var creator, height, version int

			_, err := fmt.Sscanf(t, "%d-%d-%d", &creator, &height, &version)
			if err != nil {
				return nil, err
			}

			if i == 0 {
				puCreator, puHeight, puVersion = creator, height, version
			} else {
				if _, ok := preunitHashes[[3]int{creator, height, version}]; !ok {
					return nil, gomel.NewDataError("Trying to set parent to non-existing unit")
				}
				parents = append(parents, preunitHashes[[3]int{creator, height, version}])
			}
		}
		txsEncoded := transactions.Encode([]transactions.Tx{transactions.Tx{ID: txID}})
		txsCompressed, _ := transactions.Compress(txsEncoded, 5)
		pu := NewPreunit(puCreator, parents, txsCompressed, nil)
		txID++
		preunitHashes[[3]int{puCreator, puHeight, puVersion}] = pu.Hash()
		var addingError error
		var wg sync.WaitGroup
		wg.Add(1)
		dag.AddUnit(pu, rs, func(_ gomel.Preunit, _ gomel.Unit, err error) {
			if err != nil {
				addingError = err
			}
			wg.Done()
		})
		wg.Wait()
		if addingError != nil {
			return nil, addingError
		}
	}
	return dag, nil
}

// CreateDagFromTestFile reads dag from given test file and uses factory to create the dag
func CreateDagFromTestFile(filename string, df gomel.DagFactory) (gomel.Dag, error) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	return ReadDag(reader, df)
}