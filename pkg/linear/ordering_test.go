package linear_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/linear"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

const (
	votingLevel  = 3
	piDeltaLevel = 12
)

var _ = Describe("Ordering", func() {
	var (
		ordering gomel.LinearOrdering
	)
	Describe("DecideTimingOnLevel", func() {
		Context("On empty poset on level 0", func() {
			It("should return nil", func() {
				p, err := tests.CreatePosetFromTestFile("../testdata/empty.txt", tests.NewTestPosetFactory())
				Expect(err).NotTo(HaveOccurred())
				ordering = NewOrdering(p, votingLevel, piDeltaLevel)
				Expect(ordering.DecideTimingOnLevel(0)).To(BeNil())
			})
		})
		Context("On a poset with only dealing units on level 0", func() {
			It("should return nil", func() {
				p, err := tests.CreatePosetFromTestFile("../testdata/only_dealing.txt", tests.NewTestPosetFactory())
				Expect(err).NotTo(HaveOccurred())
				ordering = NewOrdering(p, votingLevel, piDeltaLevel)
				Expect(ordering.DecideTimingOnLevel(0)).To(BeNil())
			})
		})
		Context("On a very regular poset with 4 processes and 60 units defined in regular1.txt file", func() {
			It("should decide up to 5th level", func() {
				p, err := tests.CreatePosetFromTestFile("../testdata/regular1.txt", tests.NewTestPosetFactory())
				Expect(err).NotTo(HaveOccurred())
				ordering = NewOrdering(p, votingLevel, piDeltaLevel)
				for level := 0; level < 5; level++ {
					Expect(ordering.DecideTimingOnLevel(level)).NotTo(BeNil())
				}
				Expect(ordering.DecideTimingOnLevel(5)).To(BeNil())
			})
		})
	})
})
