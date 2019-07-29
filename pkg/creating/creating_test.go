package creating_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Creating", func() {
	Describe("in a small dag", func() {
		var (
			dag gomel.Dag
			rs  gomel.RandomSource
			h1  gomel.Hash
			h2  gomel.Hash
		)
		JustBeforeEach(func() {
			rs = tests.NewTestRandomSource()
			rs.Init(dag)
		})
		Context("that is empty", func() {
			BeforeEach(func() {
				dag, _ = tests.CreateDagFromTestFile("../testdata/empty.txt", tests.NewTestDagFactory())
			})
			It("should return a dealing unit", func() {
				pu, err := NewUnit(dag, 0, dag.NProc(), []byte{}, rs, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).To(BeEmpty())
			})
			It("should return a dealing unit", func() {
				pu, err := NewUnit(dag, 3, dag.NProc(), []byte{}, rs, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(3))
				Expect(pu.Parents()).To(BeEmpty())
			})
		})

		Context("that contains a single dealing unit", func() {
			BeforeEach(func() {
				dag, _ = tests.CreateDagFromTestFile("../testdata/one_unit.txt", tests.NewTestDagFactory())
			})
			It("should return a dealing unit for a different creator", func() {
				pu, err := NewUnit(dag, 3, dag.NProc(), []byte{}, rs, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(3))
				Expect(pu.Parents()).To(BeEmpty())
			})

			It("should fail due to not enough parents for the same creator", func() {
				_, err := NewUnit(dag, 0, dag.NProc(), []byte{}, rs, false)
				Expect(err).To(MatchError("No legal parents for the unit."))
			})
		})

		Context("that contains two dealing units", func() {
			BeforeEach(func() {
				dag, _ = tests.CreateDagFromTestFile("../testdata/two_dealing.txt", tests.NewTestDagFactory())
				h1 = *dag.PrimeUnits(0).Get(0)[0].Hash()
				h2 = *dag.PrimeUnits(0).Get(1)[0].Hash()
			})
			It("should return a unit with these parents", func() {
				pu, err := NewUnit(dag, 0, dag.NProc(), []byte{}, rs, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).NotTo(BeEmpty())
				Expect(len(pu.Parents())).To(BeEquivalentTo(2))
				Expect(*pu.Parents()[0]).To(BeEquivalentTo(h1))
				Expect(*pu.Parents()[1]).To(BeEquivalentTo(h2))
			})

			It("should fail due to not enough parents when we request a prime unit", func() {
				_, err := NewUnit(dag, 0, dag.NProc(), []byte{}, rs, true)
				Expect(err).To(MatchError("No legal parents for the unit."))
			})
		})

		Context("that contains all the dealing units", func() {
			BeforeEach(func() {
				dag, _ = tests.CreateDagFromTestFile("../testdata/only_dealing.txt", tests.NewTestDagFactory())
				h1 = *dag.PrimeUnits(0).Get(0)[0].Hash()
			})
			It("should return a unit with some parents", func() {
				pu, err := NewUnit(dag, 0, dag.NProc(), []byte{}, rs, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).NotTo(BeEmpty())
				Expect(len(pu.Parents()) > 1).To(BeTrue())
				Expect(*pu.Parents()[0]).To(BeEquivalentTo(h1))
			})

			It("should return a prime unit if we request it", func() {
				pu, err := NewUnit(dag, 0, dag.NProc(), []byte{}, rs, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).NotTo(BeEmpty())
				Expect(dag.IsQuorum(len(pu.Parents()))).To(BeTrue())
				Expect(*pu.Parents()[0]).To(BeEquivalentTo(h1))
			})
		})
	})

})
