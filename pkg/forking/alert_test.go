package forking_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/adder"
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
		alerters []*AlertHandler
		dags     []gomel.Dag
		adders   []gomel.Adder
		adServs  []gomel.Service
		rss      []gomel.RandomSource
		netservs []network.Server
		pubKeys  []gomel.PublicKey
		privKeys []gomel.PrivateKey
		verKeys  []*bn256.VerificationKey
		secrKeys []*bn256.SecretKey
	)

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
		alerters = make([]*AlertHandler, nProc)
		dags = make([]gomel.Dag, nProc)
		adders = make([]gomel.Adder, nProc)
		adServs = make([]gomel.Service, nProc)
		rss = make([]gomel.RandomSource, nProc)
		netservs = tests.NewNetwork(int(nProc))
		for i := range dags {
			dags[i] = dag.New(nProc)
			rss[i] = tests.NewTestRandomSource()
			rss[i].Bind(dags[i])
			adders[i], adServs[i] = adder.New(dags[i], nil, zerolog.Nop())
			rmc := rmc.New(verKeys, secrKeys[i])
			alerters[i] = NewAlertHandler(uint16(i), dags[i], adders[i], pubKeys, rmc, netservs[i], 5*time.Second, zerolog.Nop())
			adServs[i].Start()
		}
	})

	AfterEach(func() {
		for _, s := range adServs {
			s.Stop()
		}
	})

	AcceptSomething := func(pid uint16, wg *sync.WaitGroup) {
		defer GinkgoRecover()
		conn, err := netservs[pid].Listen(4 * time.Second)
		if err != nil {
			// Might happen, the only guarantee is 2/3 of the processes get it.
			wg.Done()
			return
		}
		alerters[pid].HandleIncoming(conn, wg)
	}

	AcceptAlert := func(pid uint16, wg *sync.WaitGroup) {
		defer GinkgoRecover()
		neededResponses := 2 * (nProc - 2)
		wg.Add(int(neededResponses))
		for k := uint16(0); k < neededResponses; k++ {
			go AcceptSomething(pid, wg)
		}
	}

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

		It("Raises an alert and rejects noncommitted forking units", func() {
			forker := uint16(0)
			pu, _, err := creating.NewUnit(dags[forker], forker, []byte{}, rss[forker], true)
			Expect(err).NotTo(HaveOccurred())
			puf, _, err := creating.NewUnit(dags[forker], forker, []byte{43}, rss[forker], true)
			Expect(err).NotTo(HaveOccurred())
			pu.SetSignature(privKeys[forker].Sign(pu))
			puf.SetSignature(privKeys[forker].Sign(puf))
			dag := dags[1]
			_, err = tests.AddUnit(dag, pu)
			Expect(err).NotTo(HaveOccurred())
			wg := &sync.WaitGroup{}
			for j := uint16(1); j < nProc; j++ {
				go AcceptAlert(j, wg)
			}
			_, err = tests.AddUnit(dag, puf)
			Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
			wg.Wait()
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
			})

			It("Adds committed forking units after acquiring commitments through alerts", func() {
				wg := &sync.WaitGroup{}
				for j := uint16(1); j < nProc; j++ {
					go AcceptAlert(j, wg)
				}
				_, err := tests.AddUnit(dags[1], pus[2])
				// We cannot expect an error or lack of it here.
				// It occuring depends on whether 2 finishes its alert before 1 tries checking for the commitment.
				wg.Wait()
				// We have to start at 3 here,because we don't know whether adding 2 succeeded, see above.
				failed := 0
				for i := uint16(3); i < nProc; i++ {
					_, err = tests.AddUnit(dags[1], pus[i])
					if err != nil {
						failed++
					}
				}
				Expect(failed).To(BeNumerically("<", 2))
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

			It("Adds forks only after acquiring commitments explicitly", func() {
				wg := &sync.WaitGroup{}
				for j := uint16(1); j < nProc; j++ {
					go AcceptAlert(j, wg)
				}
				_, err := tests.AddUnit(dags[1], dealingFork2)
				Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
				wg.Wait()
				wg.Add(1)
				go AcceptSomething(2, wg)
				alerters[1].RequestCommitment(dealingFork2, 2)
				wg.Wait()
				_, err = tests.AddUnit(dags[1], dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				err = adders[1].AddUnit(childFork2, 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Adds a unit built on forks only after acquiring commitments explicitly", func() {
				unit2, _, err := creating.NewUnit(dags[2], 2, []byte{}, rss[2], true)
				Expect(err).NotTo(HaveOccurred())
				unit2.SetSignature(privKeys[2].Sign(unit2))
				_, err = tests.AddUnit(dags[2], unit2)
				Expect(err).NotTo(HaveOccurred())
				wg := &sync.WaitGroup{}
				for j := uint16(1); j < nProc; j++ {
					go AcceptAlert(j, wg)
				}
				_, err = tests.AddUnit(dags[1], dealingFork2)
				Expect(err).To(MatchError("MissingCommitment: missing commitment to fork"))
				wg.Wait()
				wg.Add(1)
				go AcceptSomething(2, wg)
				alerters[1].RequestCommitment(dealingFork2, 2)
				wg.Wait()
				_, err = tests.AddUnit(dags[1], dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				err = adders[1].AddUnit(childFork2, 0)
				Expect(err).NotTo(HaveOccurred())
				err = adders[1].AddUnit(unit2, 0)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

})
