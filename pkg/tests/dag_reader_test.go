package tests_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/tests"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

var _ = Describe("DagReader", func() {
	var dag gomel.Dag
	var err error
	Describe("CreateDagFromTestFile", func() {
		Context("On random_10_100u file", func() {
			BeforeEach(func() {
				dag, err = CreateDagFromTestFile("../testdata/dags/10/random_100u.txt", NewTestDagFactory())
			})
			It("Should return dag with 10 parents and 100 units", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(dag.NProc()).To(Equal(uint16(10)))
				Expect(countUnits(dag)).To(Equal(100))
			})
		})
		Context("On non existing file", func() {
			BeforeEach(func() {
				dag, err = CreateDagFromTestFile("blabla", NewTestDagFactory())
			})
			It("Should return dag with 10 parents and 100 units", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
	Describe("ReadDag", func() {
		Context("On some trash", func() {
			It("Should return an error", func() {
				trashString := "fdjalskjfdalkjfa"
				dag, err = ReadDag(strings.NewReader(trashString), NewTestDagFactory())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
