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

type adder struct {
	gomel.Adder
	mx           snc.Mutex
	attemptedAdd []gomel.Preunit
}

func (a *adder) AddUnit(unit gomel.Preunit) error {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, unit)
	a.mx.Unlock()
	return a.Adder.AddUnit(unit)
}

func (a *adder) AddAntichain(units []gomel.Preunit) *gomel.AggregateError {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, units...)
	a.mx.Unlock()
	return a.Adder.AddAntichain(units)
}

func (a *adder) removeDuplicates() {
	m := make(map[gomel.Hash]gomel.Preunit)
	for _, pu := range a.attemptedAdd {
		m[*pu.Hash()] = pu
	}
	a.attemptedAdd = nil
	for _, pu := range m {
		a.attemptedAdd = append(a.attemptedAdd, pu)
	}
}

var _ = Describe("Protocol", func() {

	var (
		dags     []gomel.Dag
		adders   []*adder
		servs    []sync.Server
		netservs []network.Server
	)

	BeforeEach(func() {
		// Length 2 because the tests below only check communication between the first two processes.
		// The protocol chooses who to synchronise with at random, so this is the only way to be sure.
		netservs = tests.NewNetwork(4)
	})

	JustBeforeEach(func() {
		adders = nil
		for _, dag := range dags {
			adders = append(adders, &adder{tests.NewAdder(dag), snc.Mutex{}, nil})
		}
		servs = make([]sync.Server, 4)
		for i := 0; i < 4; i++ {
			servs[i], _ = NewServer(uint16(i), dags[i], adders[i], netservs[i], time.Second, zerolog.Nop(), 1, 3)
			servs[i].Start()
		}

	})

	Describe("in a small dag", func() {

		Context("when all dags are empty", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				for i := 0; i < 4; i++ {
					tdag, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			It("should not add anything", func() {
				time.Sleep(time.Millisecond * 200)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
				}
				for i := 0; i < 4; i++ {
					adders[i].removeDuplicates()
				}
				for i := 0; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(BeEmpty())
				}
			})
		})

		Context("when the first copy contains a single dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/one_unit.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				theUnit = tdag.MaximalUnitsPerProcess().Get(0)[0]
				for i := 1; i < 4; i++ {
					tdag, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			It("should add the unit to the second copy", func() {
				time.Sleep(time.Millisecond * 200)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
				}
				for i := 0; i < 4; i++ {
					adders[i].removeDuplicates()
				}
				Expect(adders[0].attemptedAdd).To(BeEmpty())
				for i := 1; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(HaveLen(1))
					Expect(adders[i].attemptedAdd[0].Parents()).To(HaveLen(0))
					Expect(adders[i].attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
					Expect(adders[i].attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				}
			})

		})

		Context("when the second copy contains a single dealing unit", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				tdag, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/other_unit.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				tdag, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/other_unit.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				tdag, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/other_unit.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
			})

			It("should add the unit to the first copy", func() {
				time.Sleep(time.Millisecond * 200)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
				}
				for i := 0; i < 4; i++ {
					adders[i].removeDuplicates()
				}
				Expect(adders[0].attemptedAdd).To(HaveLen(1))
				Expect(adders[0].attemptedAdd[0].Parents()).To(HaveLen(0))
				Expect(adders[0].attemptedAdd[0].Creator()).To(BeNumerically("==", 1))
				for i := 1; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(BeEmpty())
				}
			})

		})

		Context("when all copies contain all the dealing units", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				for i := 0; i < 4; i++ {
					tdag, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/only_dealing.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			It("should not add anything", func() {
				time.Sleep(time.Millisecond * 200)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
				}
				for i := 0; i < 4; i++ {
					adders[i].removeDuplicates()
				}
				for i := 0; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(BeEmpty())
				}
			})

		})
		Context("when one copy has 60 units and others are empty", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/regular.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				for i := 1; i < 4; i++ {
					tdag, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			It("should add everything", func() {
				time.Sleep(time.Millisecond * 200)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
				}
				for i := 0; i < 4; i++ {
					adders[i].removeDuplicates()
				}
				Expect(adders[0].attemptedAdd).To(BeEmpty())
				for i := 1; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(HaveLen(60))
				}
			})
		})
		Context("when trolled by a forker", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				tdag, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/exchange_with_fork_local_view1.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				for i := 1; i < 4; i++ {
					tdag, _ = tests.CreateDagFromTestFile("../../testdata/dags/4/exchange_with_fork_local_view2.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			// This behaviour is expected by the current design of the protocol.
			// However this gives an opportunity to a malicious node to enforce
			// huge exchanges between honest nodes.
			It("should add all units", func() {
				time.Sleep(time.Millisecond * 200)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
				}
				for i := 0; i < 4; i++ {
					adders[i].removeDuplicates()
				}
				for i := 1; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(HaveLen(3))
				}
			})
		})

	})

})
