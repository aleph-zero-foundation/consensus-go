package tests_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/tests"

	. "github.com/onsi/ginkgo"
	//	. "github.com/onsi/gomega"
	"bufio"
	"fmt"
	"os"
)

var _ = Describe("Generator", func() {

	poset, _ := CreatePosetFromTestFile("../testdata/empty.txt", NewTestPosetFactory())
	fmt.Println(poset.GetNProcesses())
	out := bufio.NewWriter(os.Stdout)
	NewPosetWriter().WritePoset(out, poset)
	out.Flush()
})
