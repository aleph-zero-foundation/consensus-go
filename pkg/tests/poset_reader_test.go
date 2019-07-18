package tests_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/tests"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"strings"
)

var _ = Describe("DagReader", func() {
	var p gomel.Dag
	var err error
	Describe("CreateDagFromTestFile", func() {
		Context("On random_10_100u_2par file", func() {
			BeforeEach(func() {
				p, err = CreateDagFromTestFile("../testdata/random_10p_100u_2par.txt", NewTestDagFactory())
			})
			It("Should return dag with 10 parents and 100 units", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(p.NProc()).To(Equal(10))
				Expect(countUnits(p)).To(Equal(100))
			})
		})
		Context("On non existing file", func() {
			BeforeEach(func() {
				p, err = CreateDagFromTestFile("blabla", NewTestDagFactory())
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
				p, err = ReadDag(strings.NewReader(trashString), NewTestDagFactory())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
