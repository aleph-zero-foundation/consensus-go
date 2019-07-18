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
		dag            []gomel.Dag
		rs             []gomel.RandomSource
		shareProviders map[int]bool
		delt           []byte
		err            error
	)
	BeforeEach(func() {
		n = 4
		maxLevel = 7
		pid = 0
		dag = make([]gomel.Dag, n)
		rs = make([]gomel.RandomSource, n)
		shareProviders = make(map[int]bool)
		for pid := 0; pid < n-n/3; pid++ {
			shareProviders[pid] = true
		}
		delt = tcoin.Deal(n, n/3+1)

		for pid := 0; pid < n; pid++ {
			dag[pid], err = tests.CreateDagFromTestFile("../../testdata/empty4.txt", tests.NewTestDagFactory())
			Expect(err).NotTo(HaveOccurred())
			tc, tcErr := tcoin.Decode(delt, pid)
			Expect(tcErr).NotTo(HaveOccurred())
			rs[pid] = NewCoin(dag[pid], pid, tc, shareProviders)
		}
		// Generating very regular dag
		for level := 0; level < maxLevel; level++ {
			for creator := 0; creator < n; creator++ {
				pu, err := creating.NewUnit(dag[creator], creator, 2*(n/3)+1, []byte{}, rs[creator], false)
				Expect(err).NotTo(HaveOccurred())
				for pid := 0; pid < n; pid++ {
					var wg sync.WaitGroup
					wg.Add(1)
					var added gomel.Unit
					dag[pid].AddUnit(pu, rs[pid], func(_ gomel.Preunit, u gomel.Unit, err error) {
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
				perm := rs[0].GetCRP(3)
				Expect(len(perm)).To(Equal(dag[0].NProc()))
				elems := make(map[int]bool)
				for _, pid := range perm {
					elems[pid] = true
				}
				Expect(len(elems)).To(Equal(dag[0].NProc()))
			})
			It("Should return the same permutation for all pid", func() {
				perm := make([][]int, n)
				for pid := 0; pid < n; pid++ {
					perm[pid] = rs[pid].GetCRP(2)
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
	Describe("CheckCompliance", func() {
		Context("on a prime unit created by a share provider", func() {
			Context("without random source data", func() {
				It("should return an error", func() {
					u := dag[0].PrimeUnits(2).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("with inncorrect share", func() {
				It("should return an error", func() {
					u := dag[0].PrimeUnits(2).Get(0)[0]
					v := dag[0].PrimeUnits(3).Get(0)[0]
					um := newUnitMock(u, v.RandomSourceData())
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
				})
			})
		})
		Context("on a unit not created by a share provider", func() {
			Context("with random source data", func() {
				It("should return an error", func() {
					u := dag[0].PrimeUnits(2).Get(n - 1)[0]
					um := newUnitMock(u, []byte{1, 2, 3})
					err := rs[0].CheckCompliance(um)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})

type unitMock struct {
	u      gomel.Unit
	rsData []byte
}

func newUnitMock(u gomel.Unit, rsData []byte) *unitMock {
	return &unitMock{
		u:      u,
		rsData: rsData,
	}
}

func (um *unitMock) Creator() int {
	return um.u.Creator()
}

func (um *unitMock) Signature() gomel.Signature {
	return um.u.Signature()
}

func (um *unitMock) Hash() *gomel.Hash {
	return um.u.Hash()
}

func (um *unitMock) Data() []byte {
	return um.u.Data()
}

func (um *unitMock) RandomSourceData() []byte {
	return um.rsData
}

func (um *unitMock) Height() int {
	return um.u.Height()
}

func (um *unitMock) Parents() []gomel.Unit {
	return um.u.Parents()
}

func (um *unitMock) Level() int {
	return um.u.Level()
}

func (um *unitMock) Below(v gomel.Unit) bool {
	return um.u.Below(v)
}

func (um *unitMock) Above(v gomel.Unit) bool {
	return um.u.Above(v)
}

func (um *unitMock) HasForkingEvidence(creator int) bool {
	return um.u.HasForkingEvidence(creator)
}

func (um *unitMock) Floor() [][]gomel.Unit {
	return um.u.Floor()
}
