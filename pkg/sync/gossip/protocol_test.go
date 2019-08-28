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

type dag struct {
	*tests.Dag
	attemptedAdd []gomel.Preunit
}

func (dag *dag) AddUnit(unit gomel.Preunit, rs gomel.RandomSource, callback gomel.Callback) {
	dag.attemptedAdd = append(dag.attemptedAdd, unit)
	dag.Dag.AddUnit(unit, rs, callback)
}

var _ = Describe("Protocol", func() {

	var (
		dag1   *dag
		dag2   *dag
		rs1    gomel.RandomSource
		rs2    gomel.RandomSource
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
		proto1 = NewProtocol(0, dag1, rs1, servs[0], NewDefaultPeerSource(2, 0), gomel.NopCallback, time.Second, zerolog.Nop())
		proto2 = NewProtocol(1, dag2, rs2, servs[1], NewDefaultPeerSource(2, 1), gomel.NopCallback, time.Second, zerolog.Nop())
	})

	Describe("in a small dag", func() {

		Context("when both copies are empty", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{
					Dag:          tdag1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{
					Dag:          tdag2.(*tests.Dag),
					attemptedAdd: nil,
				}
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
				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(dag2.attemptedAdd).To(BeEmpty())
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{
					Dag:          tdag1.(*tests.Dag),
					attemptedAdd: nil,
				}
				theUnit = tdag1.MaximalUnitsPerProcess().Get(0)[0]
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{
					Dag:          tdag2.(*tests.Dag),
					attemptedAdd: nil,
				}
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
				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(dag2.attemptedAdd).To(HaveLen(1))
				Expect(dag2.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(dag2.attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
				Expect(dag2.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{
					Dag:          tdag1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/other_unit.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{
					Dag:          tdag2.(*tests.Dag),
					attemptedAdd: nil,
				}
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
				Expect(dag2.attemptedAdd).To(BeEmpty())
				Expect(dag1.attemptedAdd).To(HaveLen(1))
				Expect(dag1.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(dag1.attemptedAdd[0].Creator()).To(BeNumerically("==", 1))
			})

		})

		Context("when both copies contain all the dealing units", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/only_dealing.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{
					Dag:          tdag1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tdag2 := tdag1
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{
					Dag:          tdag2.(*tests.Dag),
					attemptedAdd: nil,
				}
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
				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(dag2.attemptedAdd).To(BeEmpty())
			})

		})

	})

})
