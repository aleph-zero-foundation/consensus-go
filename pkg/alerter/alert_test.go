package alerter_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	. "gitlab.com/alephledger/consensus-go/pkg/alerter"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

var _ = Describe("Alert", func() {

	var (
		nProc    uint16
		alerters []*Alerter
		dags     []gomel.Dag
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
		alerters = make([]*Alerter, nProc)
		dags = make([]gomel.Dag, nProc)
		rss = make([]gomel.RandomSource, nProc)
		netservs = tests.NewNetwork(int(nProc))
		for i := range dags {
			dag := dag.New(nProc)
			rss[i] = tests.NewTestRandomSource()
			dag = rss[i].Bind(dag)
			rmc := rmc.New(verKeys, secrKeys[i])
			alerters[i] = New(uint16(i), dag, pubKeys, rmc, netservs[i], 5*time.Second, zerolog.Nop())
			dags[i] = Wrap(dag, alerters[i])
		}
	})

	AcceptSomething := func(pid uint16, wg *sync.WaitGroup) {
		defer GinkgoRecover()
		conn, err := netservs[pid].Listen(30 * time.Second)
		Expect(err).NotTo(HaveOccurred())
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
					_, err = gomel.AddUnit(dag, pu)
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
			_, err = gomel.AddUnit(dag, pu)
			Expect(err).NotTo(HaveOccurred())
			wg := &sync.WaitGroup{}
			for j := uint16(1); j < nProc; j++ {
				go AcceptAlert(j, wg)
			}
			_, err = gomel.AddUnit(dag, puf)
			Expect(err).To(MatchError("MissingDataError: commitment to fork"))
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
					_, err = gomel.AddUnit(dags[i], pu)
					Expect(err).NotTo(HaveOccurred())
					pus[i] = pu
				}
			})

			It("Adds committed forking units after acquiring commitments through alerts", func() {
				wg := &sync.WaitGroup{}
				for j := uint16(1); j < nProc; j++ {
					go AcceptAlert(j, wg)
				}
				_, err := gomel.AddUnit(dags[1], pus[2])
				// We cannot expect an error or lack of it here.
				// It occuring depends on whether 2 finishes its alert before 1 tries checking for the commitment.
				wg.Wait()
				// We have to start at 3 here,because we don't know whether adding 2 succeeded, see above.
				for i := uint16(3); i < nProc; i++ {
					_, err = gomel.AddUnit(dags[1], pus[i])
					Expect(err).NotTo(HaveOccurred())
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
				pu, _, err := creating.NewUnit(dags[i], i, []byte{}, rss[i], true)
				Expect(err).NotTo(HaveOccurred())
				pu.SetSignature(privKeys[i].Sign(pu))
				dealing[i] = pu
				for _, dag := range dags {
					_, err = gomel.AddUnit(dag, pu)
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
					_, err = gomel.AddUnit(forkHelpDag, dealing[i])
					Expect(err).NotTo(HaveOccurred())
				}
				dealingFork1, _, err = creating.NewUnit(dags[forker], forker, []byte{}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				dealingFork1.SetSignature(privKeys[forker].Sign(dealingFork1))
				_, err = gomel.AddUnit(dags[forker], dealingFork1)
				Expect(err).NotTo(HaveOccurred())
				dealingFork2, _, err = creating.NewUnit(forkHelpDag, forker, []byte{43}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				dealingFork2.SetSignature(privKeys[forker].Sign(dealingFork2))
				_, err = gomel.AddUnit(forkHelpDag, dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				childFork1, _, err = creating.NewUnit(dags[forker], forker, []byte{}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				childFork1.SetSignature(privKeys[forker].Sign(childFork1))
				_, err = gomel.AddUnit(dags[forker], childFork1)
				Expect(err).NotTo(HaveOccurred())
				childFork2, _, err = creating.NewUnit(forkHelpDag, forker, []byte{43}, rss[forker], true)
				Expect(err).NotTo(HaveOccurred())
				childFork2.SetSignature(privKeys[forker].Sign(childFork2))
				_, err = gomel.AddUnit(forkHelpDag, childFork2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Adds forks only after acquiring commitments explicitly", func() {
				_, err := gomel.AddUnit(dags[1], dealingFork1)
				Expect(err).NotTo(HaveOccurred())
				_, err = gomel.AddUnit(dags[1], childFork1)
				Expect(err).NotTo(HaveOccurred())
				_, err = gomel.AddUnit(dags[2], dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				_, err = gomel.AddUnit(dags[2], childFork2)
				Expect(err).NotTo(HaveOccurred())
				wg := &sync.WaitGroup{}
				for j := uint16(1); j < nProc; j++ {
					go AcceptAlert(j, wg)
				}
				_, err = gomel.AddUnit(dags[1], dealingFork2)
				Expect(err).To(MatchError("MissingDataError: commitment to fork"))
				wg.Wait()
				wg.Add(1)
				go AcceptSomething(2, wg)
				alerters[1].RequestCommitment(dealingFork2.Hash(), 2)
				wg.Wait()
				_, err = gomel.AddUnit(dags[1], dealingFork2)
				Expect(err).NotTo(HaveOccurred())
				_, err = gomel.AddUnit(dags[1], childFork2)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

})
