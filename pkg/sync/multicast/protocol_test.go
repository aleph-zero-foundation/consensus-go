package multicast_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/network"
	ctests "gitlab.com/alephledger/core-go/pkg/tests"
)

type testServer interface {
	In()
	Out(uint16)
}

type adder struct {
	gomel.Adder
	attemptedAdd []gomel.Preunit
}

func (a *adder) AddUnit(unit gomel.Preunit, source uint16) error {
	a.attemptedAdd = append(a.attemptedAdd, unit)
	return a.Adder.AddUnit(unit, source)
}

func (a *adder) AddAntichain(units []gomel.Preunit, source uint16) *gomel.AggregateError {
	a.attemptedAdd = append(a.attemptedAdd, units...)
	return a.Adder.AddUnits(units, source)
}

var _ = Describe("Protocol", func() {

	var (
		dags     []gomel.Dag
		adders   []*adder
		servs    []sync.Server
		tservs   []testServer
		netservs []network.Server
		pu       gomel.Preunit
	)

	BeforeEach(func() {
		netservs = ctests.NewNetwork(4)
	})

	AfterEach(func() {
		ctests.CloseNetwork(netservs)
	})

	JustBeforeEach(func() {
		adders = nil
		for _, dag := range dags {
			adders = append(adders, &adder{tests.NewAdder(dag), nil})
		}
		for i := 0; i < 4; i++ {
			serv := NewServer(uint16(i), dags[i], adders[i], netservs[i], time.Second, zerolog.Nop())
			servs = append(servs, serv)
			tservs = append(tservs, serv.(testServer))
		}
	})

	Describe("in a small dag", func() {

		Context("when multicasting a single dealing unit to empty dags", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				for i := 0; i < 4; i++ {
					dag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
					dags = append(dags, dag)
				}
				pu = tests.NewPreunit(0, gomel.EmptyCrown(4), []byte{}, nil)
			})

			It("should add the unit to empty copies", func() {
				for i := uint16(1); i < 4; i++ {
					go tservs[0].Out(i)
				}
				adders[0].AddUnit(pu, 0)
				for i := 1; i < 4; i++ {
					tservs[i].In()
				}
				for i := 0; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(HaveLen(1))
					Expect(adders[i].attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
					Expect(adders[i].attemptedAdd[0].Hash()).To(Equal(pu.Hash()))
				}
			})
		})
	})
})
