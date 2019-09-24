package gossip_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type adder struct {
	gomel.Adder
	attemptedAdd []gomel.Preunit
}

func (a *adder) AddUnit(unit gomel.Preunit) error {
	a.attemptedAdd = append(a.attemptedAdd, unit)
	return a.Adder.AddUnit(unit)
}

func (a *adder) AddAntichain(units []gomel.Preunit) *gomel.AggregateError {
	a.attemptedAdd = append(a.attemptedAdd, units...)
	return a.Adder.AddAntichain(units)
}

var _ = Describe("Protocol", func() {

	var (
		dag1     gomel.Dag
		dag2     gomel.Dag
		adder1   *adder
		adder2   *adder
		serv1    sync.Server
		serv2    sync.Server
		netservs []network.Server
	)

	BeforeEach(func() {
		// Length 2 because the tests below only check communication between the first two processes.
		// The protocol chooses who to synchronise with at random, so this is the only way to be sure.
		netservs = tests.NewNetwork(2)
	})

	JustBeforeEach(func() {
		adder1 = &adder{tests.NewAdder(dag1), nil}
		adder2 = &adder{tests.NewAdder(dag2), nil}
		serv1, _ = NewServer(0, dag1, adder1, netservs[0], time.Second, zerolog.Nop(), 1, 0)
		serv2, _ = NewServer(1, dag2, adder2, netservs[1], time.Second, zerolog.Nop(), 0, 1)
		serv1.Start()
		serv2.Start()
	})

	Describe("in a small dag", func() {

		Context("when both copies are empty", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
			})

			It("should not add anything", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(adder2.attemptedAdd).To(BeEmpty())
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/one_unit2.txt", tests.NewTestDagFactory())
				theUnit = dag1.MaximalUnitsPerProcess().Get(0)[0]
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
			})

			It("should add the unit to the second copy", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(adder2.attemptedAdd).To(HaveLen(1))
				Expect(adder2.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(adder2.attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
				Expect(adder2.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/other_unit2.txt", tests.NewTestDagFactory())
			})

			It("should add the unit to the first copy", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(adder2.attemptedAdd).To(BeEmpty())
				Expect(adder1.attemptedAdd).To(HaveLen(1))
				Expect(adder1.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(adder1.attemptedAdd[0].Creator()).To(BeNumerically("==", 1))
			})

		})

		Context("when both copies contain all the dealing units", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/only_dealing2.txt", tests.NewTestDagFactory())
				dag2 = dag1
			})

			It("should not add anything", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(adder2.attemptedAdd).To(BeEmpty())
			})

		})
		Context("when one copy is empty and the other has 60 units", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
			})

			It("should add everything", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto1.In()
					wg.Done()
				}()
				go func() {
					proto2.Out()
					wg.Done()
				}()
				wg.Wait()
				Expect(adder1.attemptedAdd).To(HaveLen(60))
				Expect(adder2.attemptedAdd).To(BeEmpty())
			})
		})
		Context("when trolled by a forker", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/exchange_with_fork_local_view1.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/exchange_with_fork_local_view2.txt", tests.NewTestDagFactory())
			})

			// This behaviour is expected by the current design of the protocol.
			// However this gives an opportunity to a malicious node to enforce
			// huge exchanges between honest nodes.
			It("should add all units", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto1.In()
					wg.Done()
				}()
				go func() {
					proto2.Out()
					wg.Done()
				}()
				wg.Wait()
				Expect(adder1.attemptedAdd).To(HaveLen(3))
				Expect(adder2.attemptedAdd).To(HaveLen(3))
			})
		})

	})

})
