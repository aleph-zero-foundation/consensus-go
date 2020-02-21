package coin_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/random/coin"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
	"gitlab.com/alephledger/core-go/pkg/core"
)

var _ = Describe("Coin", func() {
	var (
		n              uint16
		maxLevel       int
		seed           int
		cnfs           []config.Config
		epoch          gomel.EpochID
		dags           []gomel.Dag
		rs             []gomel.RandomSource
		rsf            []gomel.RandomSourceFactory
		sks            []gomel.PrivateKey
		pks            []gomel.PublicKey
		shareProviders map[uint16]bool
		err            error
		u              gomel.Unit
		parents        []gomel.Unit
	)
	BeforeEach(func() {
		n = 4
		epoch = gomel.EpochID(0)
		maxLevel = 7
		seed = 2137
		cnfs = make([]config.Config, n)
		dags = make([]gomel.Dag, n)
		rs = make([]gomel.RandomSource, n)
		rsf = make([]gomel.RandomSourceFactory, n)
		parents = make([]gomel.Unit, n)
		sks = make([]gomel.PrivateKey, n)
		pks = make([]gomel.PublicKey, n)

		shareProviders = make(map[uint16]bool)
		for i := uint16(0); i < gomel.MinimalQuorum(n); i++ {
			shareProviders[i] = true
		}

		for pid := uint16(0); pid < n; pid++ {
			pks[pid], sks[pid], err = signing.GenerateKeys()
			Expect(err).NotTo(HaveOccurred())
			cnfs[pid] = config.Empty()
			cnfs[pid].Pid = pid
			cnfs[pid].NProc = n
			cnfs[pid].CanSkipLevel = true
			cnfs[pid].OrderStartLevel = 0
			cnfs[pid].Checks = append(cnfs[pid].Checks, check.NoSelfForkingEvidence, check.ForkerMuting)
			cnfs[pid].PrivateKey = sks[pid]
		}
		for pid := uint16(0); pid < n; pid++ {
			cnfs[pid].PublicKeys = pks
			dags[pid] = dag.New(cnfs[pid], epoch)
			rsf[pid] = NewSeededCoinFactory(n, pid, seed, shareProviders)
			rs[pid] = rsf[pid].NewRandomSource(dags[pid])
		}
		// Generating very regular dag
		for level := 0; level < maxLevel; level++ {
			for creator := uint16(0); creator < n; creator++ {
				// create a unit
				if level == 0 {
					rsData, err := rsf[creator].DealingData(epoch)
					Expect(err).ToNot(HaveOccurred())
					u = unit.New(creator, epoch, parents, level, core.Data{}, rsData, sks[creator])
				} else {
					for pid := uint16(0); pid < n; pid++ {
						parents[pid] = dags[creator].UnitsOnLevel(level - 1).Get(pid)[0]
					}
					Expect(len(parents)).To(Equal(int(n)))
					rsData, err := rs[creator].DataToInclude(parents, level)
					Expect(err).ToNot(HaveOccurred())
					u = unit.New(creator, epoch, parents, level, core.Data{}, rsData, sks[creator])
				}
				// add the unit to dags
				for pid := uint16(0); pid < n; pid++ {
					dags[pid].Insert(u)
				}
			}
		}
	})
	Describe("Checking a unit", func() {
		Context("that was created by a share provider", func() {
			Context("without random source data", func() {
				It("should return an error", func() {
					u = dags[0].UnitsOnLevel(2).Get(0)[0]
					um := newUnitMock(u, []byte{})
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("with inncorrect share", func() {
				It("should return an error", func() {
					u := dags[0].UnitsOnLevel(2).Get(0)[0]
					v := dags[0].UnitsOnLevel(3).Get(0)[0]
					um := newUnitMock(u, v.RandomSourceData())
					err := dags[0].Check(um)
					Expect(err).To(HaveOccurred())
				})
			})
		})
		Context("that was not created by a share provider", func() {
			Context("with random source data", func() {
				It("should return an error", func() {
					u := dags[0].UnitsOnLevel(2).Get(n - 1)[0]
					um := newUnitMock(u, []byte{1, 2, 3})
					err := dags[0].Check(um)
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
