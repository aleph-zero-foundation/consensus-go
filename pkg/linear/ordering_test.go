package linear_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/linear"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

type commonRandomPermutation struct {
	n int
}

func (crp *commonRandomPermutation) Get(level int) []int {
	permutation := make([]int, crp.n, crp.n)
	for i := 0; i < crp.n; i++ {
		permutation[i] = (i + level) % crp.n
	}
	return permutation
}

func newCommonRandomPermutation(n int) *commonRandomPermutation {
	return &commonRandomPermutation{
		n: n,
	}
}

var _ = Describe("Ordering", func() {
	var (
		crp      CommonRandomPermutation
		ordering gomel.LinearOrdering
	)
	Describe("AttemptTimingDecision", func() {
		Context("On empty poset", func() {
			It("should return 0", func() {
				p, err := tests.CreatePosetFromTestFile("../testdata/empty.txt", tests.NewTestPosetFactory())
				Expect(err).NotTo(HaveOccurred())
				crp = newCommonRandomPermutation(p.NProc())
				ordering = NewOrdering(p, crp)
				Expect(ordering.AttemptTimingDecision()).To(Equal(0))
			})
		})
		Context("On a poset with only dealing units", func() {
			It("should return 0", func() {
				p, err := tests.CreatePosetFromTestFile("../testdata/only_dealing.txt", tests.NewTestPosetFactory())
				Expect(err).NotTo(HaveOccurred())
				ordering = NewOrdering(p, crp)
				Expect(ordering.AttemptTimingDecision()).To(Equal(0))
			})
		})
		Context("On a very regular poset with 4 processes and 60 units defined in regular1.txt file", func() {
			It("should return 5", func() {
				p, err := tests.CreatePosetFromTestFile("../testdata/regular1.txt", tests.NewTestPosetFactory())
				Expect(err).NotTo(HaveOccurred())
				crp = newCommonRandomPermutation(p.NProc())
				ordering = NewOrdering(p, crp)
				Expect(ordering.AttemptTimingDecision()).To(Equal(5))
			})
		})
	})
})
