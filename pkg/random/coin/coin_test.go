package coin_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Coin", func() {
	var (
		n              uint16
		maxLevel       int
		seed           int
		dag            []gomel.Dag
		rs             []gomel.RandomSource
		shareProviders map[uint16]bool
		err            error
	)
	BeforeEach(func() {
		n = 4
		maxLevel = 7
		seed = 2137
		dag = make([]gomel.Dag, n)
		rs = make([]gomel.RandomSource, n)

		shareProviders = make(map[uint16]bool)
		for i := uint16(0); i < n-n/3; i++ {
			shareProviders[i] = true
		}

		for pid := uint16(0); pid < n; pid++ {
			dag[pid], err = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
			Expect(err).NotTo(HaveOccurred())
			rs[pid] = NewFixedCoin(n, pid, seed, shareProviders)
			dag[pid] = rs[pid].Bind(dag[pid])
		}
		// Generating very regular dag
		for level := 0; level < maxLevel; level++ {
			for creator := uint16(0); creator < n; creator++ {
				pu, _, err := creating.NewUnit(dag[creator], creator, []byte{}, rs[creator], false)
				Expect(err).NotTo(HaveOccurred())
				for pid := uint16(0); pid < n; pid++ {
					_, err = gomel.AddUnit(dag[pid], pu)
					Expect(err).NotTo(HaveOccurred())
				}
			}
		}
	})
	Describe("Adding a unit", func() {
		Context("that is a prime unit created by a share provider", func() {
			Context("without random source data", func() {
				It("should return an error", func() {
					u := dag[0].PrimeUnits(2).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("with inncorrect share", func() {
				It("should return an error", func() {
					u := dag[0].PrimeUnits(2).Get(0)[0]
					v := dag[0].PrimeUnits(3).Get(0)[0]
					um := newUnitMock(u, v.RandomSourceData())
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
				})
			})
		})
		Context("on a unit not created by a share provider", func() {
			Context("with random source data", func() {
				It("should return an error", func() {
					u := dag[0].PrimeUnits(2).Get(n - 1)[0]
					um := newUnitMock(u, []byte{1, 2, 3})
					err := dag[0].Check(um)
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

func (um *unitMock) Creator() uint16 {
	return um.u.Creator()
}

func (um *unitMock) Signature() gomel.Signature {
	return um.u.Signature()
}

func (um *unitMock) Hash() *gomel.Hash {
	return um.u.Hash()
}

func (um *unitMock) View() *gomel.Crown {
	return um.u.View()
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

func (um *unitMock) Floor() [][]gomel.Unit {
	return um.u.Floor()
}
