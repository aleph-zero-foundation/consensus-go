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
		adder          []gomel.Adder
		rs             []gomel.RandomSource
		shareProviders map[uint16]bool
		err            error
	)
	BeforeEach(func() {
		n = 4
		maxLevel = 7
		seed = 2137
		dag = make([]gomel.Dag, n)
		adder = make([]gomel.Adder, n)
		rs = make([]gomel.RandomSource, n)

		shareProviders = make(map[uint16]bool)
		for i := uint16(0); i < gomel.MinimalQuorum(n); i++ {
			shareProviders[i] = true
		}

		for pid := uint16(0); pid < n; pid++ {
			dag[pid], adder[pid], err = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
			Expect(err).NotTo(HaveOccurred())
			rs[pid] = NewFixedCoin(n, pid, seed, shareProviders)
			rs[pid].Bind(dag[pid])
		}
		// Generating very regular dag
		for level := 0; level < maxLevel; level++ {
			for creator := uint16(0); creator < n; creator++ {
				pu, _, err := creating.NewUnit(dag[creator], creator, []byte{}, rs[creator], false)
				Expect(err).NotTo(HaveOccurred())
				for pid := uint16(0); pid < n; pid++ {
					err = adder[pid].AddUnit(pu, pu.Creator())
					Expect(err).NotTo(HaveOccurred())
				}
			}
		}
	})
	Describe("Adding a unit", func() {
		Context("that is a prime unit created by a share provider", func() {
			Context("without random source data", func() {
				It("should return an error", func() {
					u := dag[0].UnitsOnLevel(2).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("with inncorrect share", func() {
				It("should return an error", func() {
					u := dag[0].UnitsOnLevel(2).Get(0)[0]
					v := dag[0].UnitsOnLevel(3).Get(0)[0]
					um := newUnitMock(u, v.RandomSourceData())
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
				})
			})
		})
		Context("on a unit not created by a share provider", func() {
			Context("with random source data", func() {
				It("should return an error", func() {
					u := dag[0].UnitsOnLevel(2).Get(n - 1)[0]
					um := newUnitMock(u, []byte{1, 2, 3})
					err := dag[0].Check(um)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})

type unitMock struct {
	gomel.Unit
	rsData []byte
}

func newUnitMock(u gomel.Unit, rsData []byte) *unitMock {
	return &unitMock{u, rsData}
}

func (um *unitMock) RandomSourceData() []byte {
	return um.rsData
}
