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
func ReadDag(reader io.Reader, df DagFactory) (gomel.Dag, gomel.Adder, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Scan()
	text := scanner.Text()
	var n uint16

	_, err := fmt.Sscanf(text, "%d", &n)
	if err != nil {
		return nil, nil, err
	}

	dag, adder := df.CreateDag(n)
	preunitHashes := make(map[[3]int]*gomel.Hash)

	var txID int

	for scanner.Scan() {
		text := scanner.Text()
		// skip comments
		if strings.HasPrefix(text, "//") {
			continue
		}
		var puCreator, puHeight, puVersion int
		parents := make([]*gomel.Hash, n)
		parentsHeights := make([]int, n)
		for i := uint16(0); i < n; i++ {
			parentsHeights[i] = -1
		}
		for i, t := range strings.Split(text, " ") {
			var creator, height, version int

			_, err := fmt.Sscanf(t, "%d-%d-%d", &creator, &height, &version)
			if err != nil {
				return nil, nil, err
			}

			if i == 0 {
				puCreator, puHeight, puVersion = creator, height, version
			} else {
				if _, ok := preunitHashes[[3]int{creator, height, version}]; !ok {
					return nil, nil, gomel.NewDataError("Trying to set parent to non-existing unit")
				}
				if parents[creator] != nil {
					return nil, nil, gomel.NewDataError("Duplicate parent")
				}
				parents[creator] = preunitHashes[[3]int{creator, height, version}]
				parentsHeights[creator] = height
			}
		}
		unitData := make([]byte, 4)
		binary.LittleEndian.PutUint32(unitData, uint32(txID))
		pu := NewPreunit(uint16(puCreator), gomel.NewCrown(parentsHeights, gomel.CombineHashes(parents)), unitData, nil, nil)
		txID++
		preunitHashes[[3]int{puCreator, puHeight, puVersion}] = pu.Hash()
		err := adder.AddPreunits(pu.Creator(), pu)[0]
		if err != nil {
			return nil, nil, err
		}
	}
	return dag, adder, nil
}

// CreateDagFromTestFile reads a dag description from the given test file and uses the factory to build the dag.
func CreateDagFromTestFile(filename string, df DagFactory) (gomel.Dag, gomel.Adder, error) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return nil, nil, err
	}
	reader := bufio.NewReader(file)
	return ReadDag(reader, df)
}
