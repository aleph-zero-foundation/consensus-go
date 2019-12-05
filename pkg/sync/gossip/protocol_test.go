package gossip_test

import (
	snc "sync"
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

type testServer interface {
	In()
	Out()
}

type adder struct {
	gomel.Adder
	mx           snc.Mutex
	attemptedAdd []gomel.Preunit
}

func (a *adder) AddUnit(unit gomel.Preunit, source uint16) error {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, unit)
	a.mx.Unlock()
	return a.Adder.AddUnit(unit, source)
}

func (a *adder) AddUnits(units []gomel.Preunit, source uint16) *gomel.AggregateError {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, units...)
	a.mx.Unlock()
	return a.Adder.AddUnits(units, source)
}

func (a *adder) removeDuplicates() {
	m := make(map[gomel.Hash]gomel.Preunit)
	a.mx.Lock()
	defer a.mx.Unlock()
	for _, pu := range a.attemptedAdd {
		m[*pu.Hash()] = pu
	}
	a.attemptedAdd = nil
	for _, pu := range m {
		a.attemptedAdd = append(a.attemptedAdd, pu)
	}
}

func (a *adder) added() []gomel.Preunit {
	a.mx.Lock()
	defer a.mx.Unlock()
	return a.attemptedAdd
}

var _ = Describe("Protocol", func() {

	var (
		dags     []gomel.Dag
		adders   []*adder
		servs    []sync.Server
		requests []chan<- uint16
		tservs   []testServer
		netservs []network.Server
	)

	BeforeEach(func() {
		// Length 2 because the tests below only check communication between the first two processes.
		netservs = tests.NewNetwork(2)
	})

	AfterEach(func() {
		tests.CloseNetwork(netservs)
	})

	JustBeforeEach(func() {
		adders = nil
		for _, dag := range dags {
			adders = append(adders, &adder{Adder: tests.NewAdder(dag)})
		}
		servs = make([]sync.Server, 2)
		requests = make([]chan<- uint16, 2)
		tservs = make([]testServer, 2)
		for i := 0; i < 2; i++ {
			servs[i], requests[i] = NewServer(uint16(i), dags[i], adders[i], netservs[i], time.Second, zerolog.Nop(), 1, 3)
			tservs[i] = servs[i].(testServer)
		}
	})

	Describe("in a small dag", func() {

		Context("when all dags are empty", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				for i := 0; i < 2; i++ {
					tdag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			It("should not add anything", func() {
				go tservs[0].Out()
				requests[0] <- 1
				tservs[1].In()
				for i := 0; i < 2; i++ {
					adders[i].removeDuplicates()
				}
				for i := 0; i < 2; i++ {
					Expect(adders[i].added()).To(BeEmpty())
				}
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/one_unit.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				theUnit = tdag.MaximalUnitsPerProcess().Get(0)[0]
				tdag, _, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
			})

			It("should add the unit to the second copy", func() {
				go tservs[0].Out()
				requests[0] <- 1
				tservs[1].In()
				for i := 0; i < 2; i++ {
					adders[i].removeDuplicates()
				}
				Expect(adders[0].added()).To(BeEmpty())
				Expect(adders[1].added()).To(HaveLen(1))
				Expect(adders[1].added()[0].Creator()).To(BeNumerically("==", 0))
				Expect(adders[1].added()[0].Hash()).To(Equal(theUnit.Hash()))
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				tdag, _, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/other_unit.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
			})

			It("should add the unit to the first copy", func() {
				go tservs[1].In()
				requests[0] <- 1
				tservs[0].Out()
				for i := 0; i < 2; i++ {
					adders[i].removeDuplicates()
				}
				Expect(adders[0].added()).To(HaveLen(1))
				Expect(adders[0].added()[0].Creator()).To(BeNumerically("==", 1))
				Expect(adders[1].added()).To(BeEmpty())
			})

		})

		Context("when all copies contain all the dealing units", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				for i := 0; i < 2; i++ {
					tdag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/only_dealing.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			It("should not add anything", func() {
				go tservs[0].Out()
				requests[0] <- 1
				tservs[1].In()
				for i := 0; i < 2; i++ {
					adders[i].removeDuplicates()
				}
				for i := 0; i < 2; i++ {
					Expect(adders[i].added()).To(BeEmpty())
				}
			})

		})
		Context("when one copy is empty and the other has 44 units", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				tdag, _, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
			})

			It("should add everything", func() {
				go tservs[0].Out()
				requests[0] <- 1
				tservs[1].In()
				for i := 0; i < 2; i++ {
					adders[i].removeDuplicates()
				}
				Expect(adders[0].added()).To(BeEmpty())
				Expect(adders[1].added()).To(HaveLen(44))
			})
		})
		Context("when trolled by a forker", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/exchange_with_fork_local_view1.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				tdag, _, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/exchange_with_fork_local_view2.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
			})

			It("should add all units", func() {
				go tservs[0].Out()
				requests[0] <- 1
				tservs[1].In()
				for i := 0; i < 2; i++ {
					adders[i].removeDuplicates()
				}
				for i := 0; i < 2; i++ {
					Expect(adders[i].added()).To(HaveLen(3))
				}
			})
		})
	})

})
