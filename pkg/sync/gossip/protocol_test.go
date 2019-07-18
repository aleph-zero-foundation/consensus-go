package gossip_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type dag struct {
	*tests.Dag
	attemptedAdd []gomel.Preunit
}

func (p *dag) AddUnit(unit gomel.Preunit, rs gomel.RandomSource, callback func(gomel.Preunit, gomel.Unit, error)) {
	p.attemptedAdd = append(p.attemptedAdd, unit)
	p.Dag.AddUnit(unit, rs, callback)
}

var _ = Describe("Protocol", func() {

	var (
		p1     *dag
		p2     *dag
		rs1    gomel.RandomSource
		rs2    gomel.RandomSource
		proto1 gsync.Protocol
		proto2 gsync.Protocol
		ls     []network.Listener
		d      network.Dialer
	)

	BeforeEach(func() {
		// Length 2 because the tests below only check communication between the first two processes.
		// The protocol chooses who to synchronise with at random, so this is the only way to be sure.
		d, ls = tests.NewNetwork(2)
	})

	JustBeforeEach(func() {
		proto1 = NewProtocol(0, p1, rs1, d, ls[0], NewDefaultPeerSource(2, 0), time.Second, make(chan int), zerolog.Nop())
		proto2 = NewProtocol(1, p2, rs2, d, ls[1], NewDefaultPeerSource(2, 1), time.Second, make(chan int), zerolog.Nop())
	})

	Describe("in a small dag", func() {

		Context("when both copies are empty", func() {

			BeforeEach(func() {
				tp1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource(tp1)
				p1 = &dag{
					Dag:          tp1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tp2, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource(tp2)
				p2 = &dag{
					Dag:          tp2.(*tests.Dag),
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
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(p2.attemptedAdd).To(BeEmpty())
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				tp1, _ := tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource(tp1)
				p1 = &dag{
					Dag:          tp1.(*tests.Dag),
					attemptedAdd: nil,
				}
				theUnit = tp1.MaximalUnitsPerProcess().Get(0)[0]
				tp2, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource(tp2)
				p2 = &dag{
					Dag:          tp2.(*tests.Dag),
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
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(p2.attemptedAdd).To(HaveLen(1))
				Expect(p2.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(p2.attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
				Expect(p2.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				tp1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource(tp1)
				p1 = &dag{
					Dag:          tp1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tp2, _ := tests.CreateDagFromTestFile("../../testdata/other_unit.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource(tp2)
				p2 = &dag{
					Dag:          tp2.(*tests.Dag),
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
				Expect(p2.attemptedAdd).To(BeEmpty())
				Expect(p1.attemptedAdd).To(HaveLen(1))
				Expect(p1.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(p1.attemptedAdd[0].Creator()).To(BeNumerically("==", 1))
			})

		})

		Context("when both copies contain all the dealing units", func() {

			BeforeEach(func() {
				tp1, _ := tests.CreateDagFromTestFile("../../testdata/only_dealing.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource(tp1)
				p1 = &dag{
					Dag:          tp1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tp2 := tp1
				rs2 = tests.NewTestRandomSource(tp2)
				p2 = &dag{
					Dag:          tp2.(*tests.Dag),
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
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(p2.attemptedAdd).To(BeEmpty())
			})

		})

	})

})
