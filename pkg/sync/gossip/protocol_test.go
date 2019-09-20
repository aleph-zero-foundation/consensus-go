package gossip_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
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
		dag1   gomel.Dag
		adder1 *adder
		dag2   gomel.Dag
		adder2 *adder
		proto1 gsync.Protocol
		proto2 gsync.Protocol
		servs  []network.Server
	)

	BeforeEach(func() {
		// Length 2 because the tests below only check communication between the first two processes.
		// The protocol chooses who to synchronise with at random, so this is the only way to be sure.
		servs = tests.NewNetwork(2)
	})

	JustBeforeEach(func() {
		adder1 = &adder{tests.NewAdder(dag1), nil}
		adder2 = &adder{tests.NewAdder(dag2), nil}
		proto1 = NewProtocol(0, dag1, adder1, servs[0], NewDefaultPeerSource(2, 0), time.Second, zerolog.Nop())
		proto2 = NewProtocol(1, dag2, adder2, servs[1], NewDefaultPeerSource(2, 1), time.Second, zerolog.Nop())
	})

	Describe("in a small dag", func() {

		Context("when both copies are empty", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
			})

			It("should not add anything", func() {
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
				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(adder2.attemptedAdd).To(BeEmpty())
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/one_unit.txt", tests.NewTestDagFactory())
				theUnit = dag1.MaximalUnitsPerProcess().Get(0)[0]
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
			})

			It("should add the unit to the second copy", func() {
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
				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(adder2.attemptedAdd).To(HaveLen(1))
				Expect(adder2.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(adder2.attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
				Expect(adder2.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/other_unit.txt", tests.NewTestDagFactory())
			})

			It("should add the unit to the first copy", func() {
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
				Expect(adder2.attemptedAdd).To(BeEmpty())
				Expect(adder1.attemptedAdd).To(HaveLen(1))
				Expect(adder1.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(adder1.attemptedAdd[0].Creator()).To(BeNumerically("==", 1))
			})

		})

		Context("when both copies contain all the dealing units", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				dag2 = dag1
			})

			It("should not add anything", func() {
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
