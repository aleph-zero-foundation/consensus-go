package tests

import (
	"bufio"
	"fmt"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"io"
	"os"
	"strings"
	"sync"
)

// TestPosetReader is meant to read test posets
type TestPosetReader struct{}

// NewTestPosetReader returns instation of a PosetReader
func NewTestPosetReader() TestPosetReader {
	return TestPosetReader{}
}

// ReadPoset reads poset from the given reader and creates it using given poset factory
func (TestPosetReader) ReadPoset(reader io.Reader, pf gomel.PosetFactory) (gomel.Poset, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Scan()
	text := scanner.Text()
	var n int

	_, err := fmt.Sscanf(text, "%d", &n)
	if err != nil {
		return nil, err
	}

	p := pf.CreatePoset(n)
	preunitHashes := make(map[[3]int]gomel.Hash)

	for scanner.Scan() {
		text := scanner.Text()
		var puCreator, puHeight, puVersion int
		parents := []gomel.Hash{}
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
		pu := newPreunit(puCreator, parents)
		preunitHashes[[3]int{puCreator, puHeight, puVersion}] = *pu.Hash()
		var addingError error
		var wg sync.WaitGroup
		wg.Add(1)
		p.AddUnit(pu, func(_ gomel.Preunit, _ gomel.Unit, err error) {
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
	return p, nil
}

// CreatePosetFromTestFile reads poset from given test file and uses factory to create the poset
func CreatePosetFromTestFile(filename string, pf gomel.PosetFactory) (gomel.Poset, error) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(file)
	return NewTestPosetReader().ReadPoset(reader, pf)
}
