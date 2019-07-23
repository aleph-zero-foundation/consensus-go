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

type dag struct {
	*tests.Dag
	attemptedAdd []gomel.Preunit
}

func (dag *dag) AddUnit(unit gomel.Preunit, rs gomel.RandomSource, callback func(gomel.Preunit, gomel.Unit, error)) {
	dag.attemptedAdd = append(dag.attemptedAdd, unit)
	dag.Dag.AddUnit(unit, rs, callback)
}

var _ = Describe("Protocol", func() {

	var (
		dags    []*dag
		rs      []gomel.RandomSource
		servs   []sync.Server
		ls      []network.Listener
		d       network.Dialer
		request func(unit gomel.Unit)
		serv    sync.Server
		theUnit gomel.Unit
	)

	BeforeEach(func() {
		d, ls = tests.NewNetwork(10)
	})

	JustBeforeEach(func() {
		serv, request = NewServer(0, dags[0], rs[0], d, ls[0], time.Second, sync.Noop(), zerolog.Nop())
		servs = []sync.Server{serv}
		serv.Start()
		for i := 1; i < 10; i++ {
			serv, _ = NewServer(uint16(i), dags[i], rs[i], d, ls[i], time.Second, sync.Noop(), zerolog.Nop())
			servs = append(servs, serv)
			serv.Start()
		}
	})

	Describe("in a small dag", func() {

		Context("when multicasting a single dealing unit to empty posets", func() {

			BeforeEach(func() {
				rs = []gomel.RandomSource{}
				dags = []*dag{}

				tdag, _ := tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				rs = append(rs, tests.NewTestRandomSource(tdag))
				dags = append(dags, &dag{Dag: tdag.(*tests.Dag)})
				theUnit = tdag.MaximalUnitsPerProcess().Get(0)[0]

				for i := 1; i < 10; i++ {
					tdag, _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
					rs = append(rs, tests.NewTestRandomSource(tdag))
					dags = append(dags, &dag{Dag: tdag.(*tests.Dag)})
				}
			})

			It("should add the unit to empty copies", func() {
				request(theUnit)
				time.Sleep(time.Second)
				Expect(dags[0].attemptedAdd).To(BeEmpty())
				for i := 1; i < 10; i++ {
					Expect(dags[i].attemptedAdd).To(HaveLen(1))
					Expect(dags[i].attemptedAdd[0].Parents()).To(HaveLen(0))
					Expect(dags[i].attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
					Expect(dags[i].attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				}
			})

		})

	})

})
