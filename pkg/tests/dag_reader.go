package tests

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// ReadDag reads a dag description from the given reader and builds the dag using the given dag factory.
func ReadDag(reader io.Reader, df gomel.DagFactory) (gomel.Dag, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Scan()
	text := scanner.Text()
	var n uint16

	_, err := fmt.Sscanf(text, "%d", &n)
	if err != nil {
		return nil, err
	}

	dag := df.CreateDag(gomel.DagConfig{Keys: make([]gomel.PublicKey, n)})
	preunitHashes := make(map[[3]int]*gomel.Hash)

	var txID int

	for scanner.Scan() {
		text := scanner.Text()
		// skip comments
		if strings.HasPrefix(text, "//") {
			continue
		}
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
		unitData := make([]byte, 4)
		binary.LittleEndian.PutUint32(unitData, uint32(txID))
		pu := NewPreunit(uint16(puCreator), parents, unitData, nil)
		txID++
		preunitHashes[[3]int{puCreator, puHeight, puVersion}] = pu.Hash()
		_, err := gomel.AddUnit(dag, pu)
		if err != nil {
			return nil, err
		}
	}
	return dag, nil
}

// CreateDagFromTestFile reads a dag description from the given test file and uses the factory to build the dag.
func CreateDagFromTestFile(filename string, df gomel.DagFactory) (gomel.Dag, error) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	return ReadDag(reader, df)
}
