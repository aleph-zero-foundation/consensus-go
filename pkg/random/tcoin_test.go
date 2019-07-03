package random_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/random"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Tcoin", func() {
	var (
		pid   int
		poset gomel.Poset
		rs    gomel.RandomSource
		err   error
	)
	BeforeEach(func() {
		pid = 0
		poset, err = tests.CreatePosetFromTestFile("../testdata/empty.txt", tests.NewTestPosetFactory())
		Expect(err).NotTo(HaveOccurred())
		rs = NewTcSource(poset, pid)
	})
	Describe("GetCRP", func() {
		Context("On a given level", func() {
			It("Should return a permutation of pids", func() {
				perm := rs.GetCRP(0)
				Expect(len(perm)).To(Equal(poset.NProc()))
				elems := make(map[int]bool)
				for _, pid := range perm {
					elems[pid] = true
				}
				Expect(len(elems)).To(Equal(poset.NProc()))
			})
		})
	})
	Describe("Update", func() {
		Context("When adding a dealing unit", func() {
			Context("without a tc", func() {
				It("Should return an error", func() {
					pu := tests.NewPreunit(pid, nil, nil, []byte{0})
					err := rs.Update(pu)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("with a tc", func() {
				It("Should update without errors", func() {
					rsData := rs.DataToInclude(0, nil, 0)
					Expect(len(rsData)).NotTo(Equal(0))
					pu := tests.NewPreunit(pid, nil, nil, rsData)
					err := rs.Update(pu)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})
	// TODO: More tests
})
