package coin_test

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	. "gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Coin", func() {
	var (
		pid            int
		n              int
		maxLevel       int
		poset          []gomel.Poset
		rs             []gomel.RandomSource
		shareProviders map[int]bool
		delt           []byte
		err            error
	)
	BeforeEach(func() {
		n = 4
		maxLevel = 7
		pid = 0
		poset = make([]gomel.Poset, n)
		rs = make([]gomel.RandomSource, n)
		shareProviders = make(map[int]bool)
		for pid := 0; pid < n-n/3; pid++ {
			shareProviders[pid] = true
		}
		delt = tcoin.Deal(n, n/3+1)

		for pid := 0; pid < n; pid++ {
			poset[pid], err = tests.CreatePosetFromTestFile("../../testdata/empty4.txt", tests.NewTestPosetFactory())
			Expect(err).NotTo(HaveOccurred())
			tc, tcErr := tcoin.Decode(delt, pid)
			Expect(tcErr).NotTo(HaveOccurred())
			rs[pid] = NewCoin(poset[pid], pid, tc, shareProviders)
		}
		// Generating very regular poset
		for level := 0; level < maxLevel; level++ {
			for creator := 0; creator < n; creator++ {
				pu, err := creating.NewUnit(poset[creator], creator, 2*(n/3)+1, []byte{}, rs[creator], false)
				Expect(err).NotTo(HaveOccurred())
				for pid := 0; pid < n; pid++ {
					var wg sync.WaitGroup
					wg.Add(1)
					var added gomel.Unit
					poset[pid].AddUnit(pu, rs[pid], func(_ gomel.Preunit, u gomel.Unit, err error) {
						defer wg.Done()
						added = u
						Expect(err).NotTo(HaveOccurred())
					})
					errComp := rs[pid].CheckCompliance(added)
					Expect(errComp).NotTo(HaveOccurred())
					rs[pid].Update(added)
					wg.Wait()
				}
			}
		}
	})
	Describe("GetCRP", func() {
		Context("On a given level", func() {
			It("Should return a permutation of pids", func() {
				perm := rs[0].GetCRP(8)
				Expect(len(perm)).To(Equal(poset[0].NProc()))
				elems := make(map[int]bool)
				for _, pid := range perm {
					elems[pid] = true
				}
				Expect(len(elems)).To(Equal(poset[0].NProc()))
			})
			It("Should return the same permutation for all pid", func() {
				perm := make([][]int, n)
				for pid := 0; pid < n; pid++ {
					perm[pid] = rs[pid].GetCRP(10)
				}
				for pid := 1; pid < n; pid++ {
					for i := range perm[pid] {
						Expect(perm[pid][i]).To(Equal(perm[pid-1][i]))
					}
				}
			})
			Context("On too high level", func() {
				It("Should return nil", func() {
					perm := rs[0].GetCRP(11)
					Expect(perm).To(BeNil())
				})
			})
		})
	})
})
