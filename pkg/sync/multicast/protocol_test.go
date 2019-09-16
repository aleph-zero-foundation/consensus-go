package multicast_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
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
		dags     []gomel.Dag
		adders   []*adder
		servs    []sync.Server
		netservs []network.Server
		request  func(gomel.Unit)
		serv     sync.Server
		theUnit  gomel.Unit
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(4)
	})

	JustBeforeEach(func() {
		adders = nil
		for _, dag := range dags {
			adders = append(adders, &adder{tests.NewAdder(dag), nil})
		}
		serv, request = NewServer(0, dags[0], adders[0], netservs[0], time.Millisecond*200, sync.NopFallback(), zerolog.Nop())
		servs = []sync.Server{serv}
		serv.Start()
		for i := 1; i < 4; i++ {
			serv, _ = NewServer(uint16(i), dags[i], adders[i], netservs[i], time.Second, sync.NopFallback(), zerolog.Nop())
			servs = append(servs, serv)
			serv.Start()
		}
	})

	Describe("in a small dag", func() {

		Context("when multicasting a single dealing unit to empty posets", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}

				tdag, _ := tests.CreateDagFromTestFile("../../testdata/one_unit4.txt", tests.NewTestDagFactory())
				dags = append(dags, tdag)
				theUnit = tdag.MaximalUnitsPerProcess().Get(0)[0]

				for i := 1; i < 4; i++ {
					tdag, _ = tests.CreateDagFromTestFile("../../testdata/empty4.txt", tests.NewTestDagFactory())
					dags = append(dags, tdag)
				}
			})

			It("should add the unit to empty copies", func() {
				request(theUnit)
				time.Sleep(time.Millisecond * 500)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
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

	})

})
