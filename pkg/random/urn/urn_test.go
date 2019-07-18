package urn_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/random/urn"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Tcoin", func() {
	var (
		n
		maxLevel
		poset []gomel.Poset
		rs    []gomel.RandomSource
		err   error
		shareProviders map[int]bool
	)
	BeforeEach(func() {
		pid = 0
		poset, err = tests.CreatePosetFromTestFile("../../testdata/empty4.txt", tests.NewTestPosetFactory())
		n = 4
		maxLevel = 13
		poset = make([]gomel.Poset, n)
		rs = make([]gomel.RandomSource, n)
		for pid := 0; pid < n; pid++ {
			poset[pid], err = tests.CreatePosetFromTestFile("../../testdata/empty4.txt", tests.NewTestPosetFactory())
			Expect(err).NotTo(HaveOccurred())
		Expect(err).NotTo(HaveOccurred())
		shareProviders = make(map[int]bool);
		for i:=0; i < poset.NProc(); i++ {
			if poset.IsQuorum(i) {
				break;
			}
			shareProviders[i] = true
		}
		rs = NewUrn(poset, pid, shareProviders)
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
})
