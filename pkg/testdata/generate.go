package main

import (
	"bufio"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
	"os"
)

func writeToFile(filename string, poset gomel.Poset) error {
	file, err := os.Create(filename)
	defer file.Close()
	if err != nil {
		return err
	}
	out := bufio.NewWriter(file)
	tests.NewPosetWriter().WritePoset(out, poset)
	out.Flush()
	return nil
}

// Use this to generate more test files
func main() {
	writeToFile("empty.txt", tests.CreateRandomNonForking(10, 2, 2, 0))
	writeToFile("random_10p_100u_2par.txt", tests.CreateRandomNonForking(10, 2, 2, 100))
}
