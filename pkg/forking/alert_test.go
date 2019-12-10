package forking_test

import (
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	. "gitlab.com/alephledger/consensus-go/pkg/forking"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Alert", func() {

	var (
		nProc    uint16
		alerters []gomel.Alerter
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
			go alerters[pid].HandleIncoming(conn, &wg)
		}
		// Clean up pending alerts, assume done if timeout.
		for i := 0; i < int(nProc); i++ {
			conn, err := netservs[pid].Listen(2 * time.Second)
			if err != nil {
				return
			}
			conn.TimeoutAfter(time.Second)
			wg.Add(1)
			go alerters[pid].HandleIncoming(conn, &wg)
		}
	}

	StopHandling := func() {
		atomic.StoreInt64(&stop, 1)
		wg.Wait()
		tests.CloseNetwork(netservs)
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
		netservs = tests.NewNetwork(int(nProc))
		stop = 0
		for i := range dags {
			dags[i] = dag.New(nProc)
			rss[i] = tests.NewTestRandomSource()
			rss[i].Bind(dags[i])
			rmc := rmc.New(verKeys, secrKeys[i])
			alerters[i] = NewAlertHandler(uint16(i), dags[i], pubKeys, rmc, netservs[i], 2*time.Second, zerolog.Nop())
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
				pu, _, err := creating.NewUnit(dags[i], i, []byte{}, rss[i], true)
				Expect(err).NotTo(HaveOccurred())
				pu.SetSignature(privKeys[i].Sign(pu))
				for _, dag := range dags {
					_, err = tests.AddUnit(dag, pu)
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		It("Does not add noncommitted forking units after an alert", func() {
			forker := uint16(0)
			pu, _, err := creating.NewUnit(dags[forker], forker, []byte{}, rss[forker], true)
			Expect(err).NotTo(HaveOccurred())
			puf, _, err := creating.NewUnit(dags[forker], forker, []byte{43}, rss[forker], true)
			Expect(err).NotTo(HaveOccurred())
			pu.SetSignature(privKeys[forker].Sign(pu))
			puf.SetSignature(privKeys[forker].Sign(puf))
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
			Expect(ignorants).To(BeNumerically("<", 3))
		})

		Context("And a forker creates a fork for every process", func() {

			var (
				pus []gomel.Preunit
			)

			BeforeEach(func() {
				forker := uint16(0)
				pus = make([]gomel.Preunit, nProc)
				for i := uint16(1); i < nProc; i++ {
					pu, _, err := creating.NewUnit(dags[forker], forker, []byte{byte(i)}, rss[forker], true)
					Expect(err).NotTo(HaveOccurred())
					pu.SetSignature(privKeys[forker].Sign(pu))
					_, err = tests.AddUnit(dags[i], pu)
					Expect(err).NotTo(HaveOccurred())
					pus[i] = pu
				}
				pu, _, err := creating.NewUnit(dags[forker], forker, []byte{0}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				pu.SetSignature(privKeys[forker].Sign(pu))
				pus[0] = pu
			})

			It("Adds committed forking units after acquiring commitments through alerts", func() {
				_, err := tests.AddUnit(dags[1], pus[0])
				Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
				failed := 0
				for j := uint16(2); j < nProc; j++ {
					_, err := tests.AddUnit(dags[j], pus[1])
					if err != nil {
						failed++
					}
				}
				Expect(failed).To(BeNumerically("<", 3))
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
				pu, _, err := creating.NewUnit(dags[i], i, []byte{}, rss[i], true)
				Expect(err).NotTo(HaveOccurred())
				pu.SetSignature(privKeys[i].Sign(pu))
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
				forkHelpDag = dag.New(nProc)
				for i := uint16(1); i < nProc; i++ {
					_, err = tests.AddUnit(forkHelpDag, dealing[i])
					Expect(err).NotTo(HaveOccurred())
				}
				dealingFork1, _, err = creating.NewUnit(dags[forker], forker, []byte{}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				dealingFork1.SetSignature(privKeys[forker].Sign(dealingFork1))
				_, err = tests.AddUnit(dags[forker], dealingFork1)
				Expect(err).NotTo(HaveOccurred())
				dealingFork2, _, err = creating.NewUnit(forkHelpDag, forker, []byte{43}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				dealingFork2.SetSignature(privKeys[forker].Sign(dealingFork2))
				_, err = tests.AddUnit(forkHelpDag, dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				childFork1, _, err = creating.NewUnit(dags[forker], forker, []byte{}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				childFork1.SetSignature(privKeys[forker].Sign(childFork1))
				_, err = tests.AddUnit(dags[forker], childFork1)
				Expect(err).NotTo(HaveOccurred())
				childFork2, _, err = creating.NewUnit(forkHelpDag, forker, []byte{43}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				childFork2.SetSignature(privKeys[forker].Sign(childFork2))
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
				// No need to transform and insert, as they don't return errors anyway.
			})

			It("Adds a unit built on forks only after acquiring commitments explicitly", func() {
				unit2, _, err := creating.NewUnit(dags[2], 2, []byte{}, rss[2], true)
				Expect(err).NotTo(HaveOccurred())
				unit2.SetSignature(privKeys[2].Sign(unit2))
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
				dags[1].Insert(dags[1].Transform(fu))
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
			})
		})
	})

})
