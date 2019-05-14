package creating_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/creating"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Creating", func() {
	Describe("in a small poset", func() {
		var (
			p  gomel.Poset
			h1 gomel.Hash
			h2 gomel.Hash
		)
		Context("that is empty", func() {
			BeforeEach(func() {
				p, _ = tests.CreatePosetFromTestFile("../testdata/empty.txt", tests.NewTestPosetFactory())
			})
			It("should return a dealing unit", func() {
				pu, err := NewUnit(p, 0, p.NProc())
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).To(BeEmpty())
			})
			It("should return a dealing unit", func() {
				pu, err := NewUnit(p, 3, p.NProc())
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(3))
				Expect(pu.Parents()).To(BeEmpty())
			})
		})

		Context("that contains a single dealing unit", func() {
			BeforeEach(func() {
				p, _ = tests.CreatePosetFromTestFile("../testdata/one_unit.txt", tests.NewTestPosetFactory())
			})
			It("should return a dealing unit for a different creator", func() {
				pu, err := NewUnit(p, 3, p.NProc())
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(3))
				Expect(pu.Parents()).To(BeEmpty())
			})

			It("should fail due to not enough parents for the same creator", func() {
				_, err := NewUnit(p, 0, p.NProc())
				Expect(err).To(MatchError("No legal parents for the unit."))
			})
		})

		Context("that contains two dealing units", func() {
			BeforeEach(func() {
				p, _ = tests.CreatePosetFromTestFile("../testdata/two_dealing.txt", tests.NewTestPosetFactory())
				h1 = *p.PrimeUnits(0).Get(0)[0].Hash()
				h2 = *p.PrimeUnits(0).Get(1)[0].Hash()
			})
			It("should return a unit with these parents", func() {
				pu, err := NewUnit(p, 0, p.NProc())
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).NotTo(BeEmpty())
				Expect(len(pu.Parents())).To(BeEquivalentTo(2))
				Expect(pu.Parents()[0]).To(BeEquivalentTo(h1))
				Expect(pu.Parents()[1]).To(BeEquivalentTo(h2))
			})
		})

		Context("that contains all the dealing units", func() {
			BeforeEach(func() {
				p, _ = tests.CreatePosetFromTestFile("../testdata/only_dealing.txt", tests.NewTestPosetFactory())
				h1 = *p.PrimeUnits(0).Get(0)[0].Hash()
			})
			It("should return a unit with some parents", func() {
				pu, err := NewUnit(p, 0, p.NProc())
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).NotTo(BeEmpty())
				Expect(len(pu.Parents()) > 1).To(BeTrue())
				Expect(pu.Parents()[0]).To(BeEquivalentTo(h1))
			})
		})
	})

})
