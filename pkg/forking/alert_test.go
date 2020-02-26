package forking_test

import (
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creator"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	. "gitlab.com/alephledger/consensus-go/pkg/forking"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/core-go/pkg/network"
	ctests "gitlab.com/alephledger/core-go/pkg/tests"
)

type orderer struct {
	gomel.Orderer
	dag gomel.Dag
}

func (o *orderer) SetDag(dag gomel.Dag) {
	o.dag = dag
}

func (o *orderer) MaxUnits(epochID gomel.EpochID) gomel.SlottedUnits {
	return o.dag.MaximalUnitsPerProcess()
}

func (o *orderer) UnitsByHash(hashes ...*gomel.Hash) []gomel.Unit {
	return o.dag.GetUnits(hashes)
}

func newOrderer(dag gomel.Dag) *orderer {
	return &orderer{Orderer: tests.NewOrderer(), dag: dag}
}

func toPreunit(u gomel.Unit) gomel.Preunit {
	id := gomel.ID(u.Height(), u.Creator(), u.EpochID())
	return unit.NewPreunit(id, u.View(), u.Data(), u.RandomSourceData(), u.Signature())
}

func retrieveParentCandidates(dag gomel.Dag) []gomel.Unit {
	parents := make([]gomel.Unit, dag.NProc())
	ix := 0
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		defer func() { ix++ }()
		if len(units) > 0 {
			parents[ix] = units[0]
		}
		return true
	})
	creator.MakeConsistent(parents)
	return parents
}

func newUnit(dag gomel.Dag, creator uint16, pk gomel.PrivateKey, rss gomel.RandomSource, data core.Data) (gomel.Unit, error) {
	parents := retrieveParentCandidates(dag)
	level := gomel.LevelFromParents(parents)
	rssData, err := rss.DataToInclude(parents, level)
	if err != nil {
		return nil, err
	}

	return unit.New(creator, dag.EpochID(), parents, level, data, rssData, pk), nil
}

var _ = Describe("Alert", func() {

	var (
		nProc    uint16
		alerters []gomel.Alerter
		orderers []*orderer
		dags     []gomel.Dag
		rss      []gomel.RandomSource
		netservs []network.Server
		pubKeys  []gomel.PublicKey
		privKeys []gomel.PrivateKey
		verKeys  []*bn256.VerificationKey
		secrKeys []*bn256.SecretKey
		wg       sync.WaitGroup
		stop     int64
	)

	KeepHandling := func(pid uint16) {
		defer GinkgoRecover()
		defer wg.Done()
		for atomic.LoadInt64(&stop) == 0 {
			conn, err := netservs[pid].Listen(2 * time.Second)
			if err != nil {
				continue
			}
			conn.TimeoutAfter(time.Second)
			wg.Add(1)
			go func() {
				defer wg.Done()
				go alerters[pid].HandleIncoming(conn)
			}()
		}
		// Clean up pending alerts, assume done if timeout.
		for i := 0; i < int(nProc); i++ {
			conn, err := netservs[pid].Listen(2 * time.Second)
			if err != nil {
				return
			}
			conn.TimeoutAfter(time.Second)
			wg.Add(1)
			go func() {
				defer wg.Done()
				go alerters[pid].HandleIncoming(conn)
			}()
		}
	}

	StopHandling := func() {
		atomic.StoreInt64(&stop, 1)
		wg.Wait()
		ctests.CloseNetwork(netservs)
	}

	BeforeEach(func() {
		nProc = 10
		pubKeys = make([]gomel.PublicKey, nProc)
		privKeys = make([]gomel.PrivateKey, nProc)
		verKeys = make([]*bn256.VerificationKey, nProc)
		secrKeys = make([]*bn256.SecretKey, nProc)
		for i := range pubKeys {
			pubKeys[i], privKeys[i], _ = signing.GenerateKeys()
			verKeys[i], secrKeys[i], _ = bn256.GenerateKeys()
		}
		alerters = make([]gomel.Alerter, nProc)
		dags = make([]gomel.Dag, nProc)
		rss = make([]gomel.RandomSource, nProc)
		netservs = ctests.NewNetwork(int(nProc))
		orderers = make([]*orderer, nProc)
		stop = 0
		for i := range dags {
			cnf := config.Empty()
			cnf.NProc = nProc
			cnf.Pid = uint16(i)
			cnf.RMCPublicKeys = verKeys
			cnf.RMCPrivateKey = secrKeys[i]
			cnf.PublicKeys = pubKeys

			orderers[i] = newOrderer(nil)

			var err error
			alerters[i], err = NewAlerter(cnf, orderers[i], netservs[i], zerolog.Nop())
			dags[i] = dag.New(cnf, gomel.EpochID(0))
			rss[i] = tests.NewTestRandomSource()
			orderers[i].SetDag(dags[i])

			Expect(err).NotTo(HaveOccurred())
			wg.Add(1)
			go KeepHandling(uint16(i))
		}
	})

	AfterEach(func() {
		StopHandling()
	})

	Describe("When the dags are empty", func() {
		It("Adds nonforking units without problems", func() {
			for i := uint16(0); i < nProc; i++ {
				u, err := newUnit(dags[i], i, privKeys[i], rss[i], []byte{})
				Expect(err).NotTo(HaveOccurred())
				pu := toPreunit(u)
				for _, dag := range dags {
					_, err = tests.AddUnit(dag, pu)
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		It("Does not add noncommitted forking units after an alert", func() {
			forker := uint16(0)

			u, err := newUnit(dags[forker], forker, privKeys[forker], rss[forker], []byte{})
			Expect(err).NotTo(HaveOccurred())
			pu := toPreunit(u)

			uf, err := newUnit(dags[forker], forker, privKeys[forker], rss[forker], []byte{43})
			Expect(err).NotTo(HaveOccurred())
			puf := toPreunit(uf)

			_, err = tests.AddUnit(dags[1], pu)
			Expect(err).NotTo(HaveOccurred())
			_, err = tests.AddUnit(dags[1], puf)
			Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
			// We only know that at least 2f+1 processes received the alert, so at least they should be aware of the fork and react accordingly.
			ignorants := 0
			for j := uint16(2); j < nProc; j++ {
				if alerters[j].IsForker(forker) {
					_, err = tests.AddUnit(dags[j], puf)
					Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
				} else {
					ignorants++
				}
			}
			Expect(ignorants).To(BeNumerically("<=", (nProc/3)-1))
			// Add the correct unit everywhere to confirm that any alerts are done.
			for j := uint16(2); j < nProc; j++ {
				_, err = tests.AddUnit(dags[j], pu)
				if err != nil {
					Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
					Eventually(func() error { return alerters[j].RequestCommitment(pu, 1) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
					_, err = tests.AddUnit(dags[j], pu)
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		Context("And a forker creates a fork for every process", func() {

			var (
				pus []gomel.Preunit
			)

			BeforeEach(func() {
				forker := uint16(0)
				pus = make([]gomel.Preunit, nProc)
				for i := uint16(1); i < nProc; i++ {

					u, err := newUnit(dags[forker], forker, privKeys[forker], rss[forker], []byte{byte(i)})
					Expect(err).NotTo(HaveOccurred())
					pu := toPreunit(u)

					_, err = tests.AddUnit(dags[i], pu)
					Expect(err).NotTo(HaveOccurred())
					pus[i] = pu
				}
				u, err := newUnit(dags[forker], forker, privKeys[forker], rss[forker], []byte{0})
				Expect(err).NotTo(HaveOccurred())
				pu := toPreunit(u)
				pus[0] = pu
			})

			It("Adds committed forking units after acquiring commitments through alerts", func() {
				_, err := tests.AddUnit(dags[1], pus[0])
				Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
				failed := []uint16{}
				for j := uint16(2); j < nProc; j++ {
					_, err := tests.AddUnit(dags[j], pus[1])
					if err != nil {
						Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
						failed = append(failed, j)
					}
				}
				Expect(len(failed)).To(BeNumerically("<=", (nProc/3)-1))
				// Ensure any alerts are done by eventually adding the unit everywhere.
				for _, j := range failed {
					_, err = tests.AddUnit(dags[j], pus[1])
					if err != nil {
						Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
						Eventually(func() error { return alerters[j].RequestCommitment(pus[1], 1) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
						_, err = tests.AddUnit(dags[j], pus[1])
						Expect(err).NotTo(HaveOccurred())
					}
				}
			})
		})
	})
	Describe("When the dag contains all dealing units", func() {

		var (
			dealing []gomel.Preunit
		)

		BeforeEach(func() {
			dealing = make([]gomel.Preunit, nProc)
			for i := uint16(1); i < nProc; i++ {

				u, err := newUnit(dags[i], i, privKeys[i], rss[i], []byte{})
				Expect(err).NotTo(HaveOccurred())
				pu := toPreunit(u)
				dealing[i] = pu
				for _, dag := range dags {
					_, err = tests.AddUnit(dag, pu)
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		Context("And a forker creates two double unit forks", func() {

			var (
				dealingFork1, dealingFork2, childFork1, childFork2 gomel.Preunit
				forkHelpDag                                        gomel.Dag
			)

			BeforeEach(func() {
				var err error
				forker := uint16(0)
				forkerCnf := config.Empty()
				forkerCnf.NProc = nProc
				forkerCnf.Pid = forker
				forkerCnf.RMCPublicKeys = verKeys
				forkerCnf.RMCPrivateKey = secrKeys[forker]
				forkerCnf.PublicKeys = pubKeys

				forkHelpDag = dag.New(forkerCnf, 0)
				for i := uint16(1); i < nProc; i++ {
					_, err = tests.AddUnit(forkHelpDag, dealing[i])
					Expect(err).NotTo(HaveOccurred())
				}
				df1, err := newUnit(dags[forker], forker, privKeys[forker], rss[forker], []byte{})
				Expect(err).NotTo(HaveOccurred())
				dealingFork1 = toPreunit(df1)
				_, err = tests.AddUnit(dags[forker], dealingFork1)
				Expect(err).NotTo(HaveOccurred())

				df2, err := newUnit(forkHelpDag, forker, privKeys[forker], rss[forker], []byte{byte(43)})
				Expect(err).NotTo(HaveOccurred())
				dealingFork2 = toPreunit(df2)
				_, err = tests.AddUnit(forkHelpDag, dealingFork2)
				Expect(err).NotTo(HaveOccurred())

				cf1, err := newUnit(dags[forker], forker, privKeys[forker], rss[forker], []byte{})
				Expect(err).NotTo(HaveOccurred())
				childFork1 = toPreunit(cf1)
				_, err = tests.AddUnit(dags[forker], childFork1)
				Expect(err).NotTo(HaveOccurred())

				cf2, err := newUnit(forkHelpDag, forker, privKeys[forker], rss[forker], []byte{43})
				Expect(err).NotTo(HaveOccurred())
				childFork2 = toPreunit(cf2)
				_, err = tests.AddUnit(forkHelpDag, childFork2)
				Expect(err).NotTo(HaveOccurred())
				_, err = tests.AddUnit(dags[1], dealingFork1)
				Expect(err).NotTo(HaveOccurred())
				_, err = tests.AddUnit(dags[1], childFork1)
				Expect(err).NotTo(HaveOccurred())
				_, err = tests.AddUnit(dags[2], dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				_, err = tests.AddUnit(dags[2], childFork2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Adds forks after acquiring commitments explicitly", func() {
				_, err := tests.AddUnit(dags[1], dealingFork2)
				Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
				Eventually(func() error { return alerters[1].RequestCommitment(dealingFork2, 2) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
				_, err = tests.AddUnit(dags[1], dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				_, err = dags[1].DecodeParents(childFork2)
				Expect(err).To(HaveOccurred())
				parents := make([]gomel.Unit, 0, nProc)
				e, ok := err.(*gomel.AmbiguousParents)
				Expect(ok).To(BeTrue())
				for _, us := range e.Units {
					parent, err2 := alerters[1].Disambiguate(us, childFork2)
					Expect(err2).NotTo(HaveOccurred())
					parents = append(parents, parent)
				}
				fu := dags[1].BuildUnit(childFork2, parents)
				Expect(dags[1].Check(fu)).To(Succeed())
				// Add one of the dealing units everywhere to confirm that any alerts are done.
				for j := uint16(2); j < nProc; j++ {
					_, err = tests.AddUnit(dags[j], dealingFork1)
					if err != nil {
						Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
						Eventually(func() error { return alerters[j].RequestCommitment(dealingFork1, 1) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
						_, err = tests.AddUnit(dags[j], dealingFork1)
						Expect(err).NotTo(HaveOccurred())
					}
				}
			})

			It("Adds a unit built on forks only after acquiring commitments explicitly", func() {
				u2, err := newUnit(dags[2], 2, privKeys[2], rss[2], []byte{})
				Expect(err).NotTo(HaveOccurred())
				unit2 := toPreunit(u2)
				_, err = tests.AddUnit(dags[2], unit2)
				Expect(err).NotTo(HaveOccurred())
				_, err = tests.AddUnit(dags[1], dealingFork2)
				Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
				Eventually(func() error { return alerters[1].RequestCommitment(dealingFork2, 2) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
				_, err = tests.AddUnit(dags[1], dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				_, err = dags[1].DecodeParents(childFork2)
				Expect(err).To(HaveOccurred())
				parents := make([]gomel.Unit, 0, nProc)
				e, ok := err.(*gomel.AmbiguousParents)
				Expect(ok).To(BeTrue())
				for _, us := range e.Units {
					parent, err2 := alerters[1].Disambiguate(us, childFork2)
					Expect(err2).NotTo(HaveOccurred())
					parents = append(parents, parent)
				}
				fu := dags[1].BuildUnit(childFork2, parents)
				Expect(dags[1].Check(fu)).To(Succeed())
				dags[1].Insert(fu)
				_, err = dags[1].DecodeParents(unit2)
				Expect(err).To(HaveOccurred())
				parents = make([]gomel.Unit, 0, nProc)
				e, ok = err.(*gomel.AmbiguousParents)
				Expect(ok).To(BeTrue())
				for _, us := range e.Units {
					parent, err2 := alerters[1].Disambiguate(us, unit2)
					Expect(err2).NotTo(HaveOccurred())
					parents = append(parents, parent)
				}
				fu = dags[1].BuildUnit(unit2, parents)
				Expect(dags[1].Check(fu)).To(Succeed())
				// Add one of the dealing units everywhere to confirm that any alerts are done.
				for j := uint16(2); j < nProc; j++ {
					_, err = tests.AddUnit(dags[j], dealingFork1)
					if err != nil {
						Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
						Eventually(func() error { return alerters[j].RequestCommitment(dealingFork1, 1) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
						_, err = tests.AddUnit(dags[j], dealingFork1)
						Expect(err).NotTo(HaveOccurred())
					}
				}
			})
		})
	})

})
