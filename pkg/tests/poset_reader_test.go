package tests_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/tests"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"strings"
)

var _ = Describe("PosetReader", func() {
	var p gomel.Poset
	var err error
	Describe("CreatePosetFromTestFile", func() {
		Context("On random_10_100u_2par file", func() {
			BeforeEach(func() {
				p, err = CreatePosetFromTestFile("../testdata/random_10p_100u_2par.txt", NewTestPosetFactory())
			})
			It("Should return poset with 10 parents and 100 units", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(p.NProc()).To(Equal(10))
				Expect(countUnits(p)).To(Equal(100))
			})
		})
		Context("On non existing file", func() {
			BeforeEach(func() {
				p, err = CreatePosetFromTestFile("blabla", NewTestPosetFactory())
			})
			It("Should return poset with 10 parents and 100 units", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
	Describe("ReadPoset", func() {
		Context("On some trash", func() {
			It("Should return an error", func() {
				trashString := "fdjalskjfdalkjfa"
				p, err = NewTestPosetReader().ReadPoset(strings.NewReader(trashString), NewTestPosetFactory())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
