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
		dag1     *dag
		dag2     *dag
		rs1      gomel.RandomSource
		rs2      gomel.RandomSource
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
		serv1 = NewServer(0, dag1, rs1, netservs[0], time.Second, zerolog.Nop(), 1, 0)
		serv2 = NewServer(1, dag2, rs2, netservs[1], time.Second, zerolog.Nop(), 0, 1)
		serv1.Start()
		serv2.Start()
	})

	Describe("in a small dag", func() {

		Context("when both copies are empty", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
			})

			It("should not add anything", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(dag2.attemptedAdd).To(BeEmpty())
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/one_unit2.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				theUnit = tdag1.MaximalUnitsPerProcess().Get(0)[0]
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
			})

			It("should add the unit to the second copy", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(dag2.attemptedAdd).To(HaveLen(1))
				Expect(dag2.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(dag2.attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
				Expect(dag2.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty2.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/other_unit2.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
			})

			It("should add the unit to the first copy", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(dag2.attemptedAdd).To(BeEmpty())
				Expect(dag1.attemptedAdd).To(HaveLen(1))
				Expect(dag1.attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(dag1.attemptedAdd[0].Creator()).To(BeNumerically("==", 1))
			})

		})

		Context("when both copies contain all the dealing units", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/only_dealing2.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				tdag2 := tdag1
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
			})

			It("should not add anything", func() {
				time.Sleep(time.Millisecond * 200)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()
				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(dag2.attemptedAdd).To(BeEmpty())
			})

		})

	})

})
