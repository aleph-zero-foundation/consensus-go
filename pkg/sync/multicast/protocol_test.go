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

func (dag *dag) AddUnit(unit gomel.Preunit, rs gomel.RandomSource, callback gomel.Callback) {
	dag.attemptedAdd = append(dag.attemptedAdd, unit)
	dag.Dag.AddUnit(unit, rs, callback)
}

var _ = Describe("Protocol", func() {

	var (
		dags     []*dag
		rs       []gomel.RandomSource
		servs    []sync.MulticastServer
		netservs []network.Server
		serv     sync.MulticastServer
		theUnit  gomel.Unit
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(4)
	})

	JustBeforeEach(func() {
		for i := 0; i < 4; i++ {
			serv = NewServer(uint16(i), dags[i], rs[i], netservs[i], time.Millisecond*200, zerolog.Nop())
			servs = append(servs, serv)
			serv.Start()
		}
	})

	Describe("in a small dag", func() {

		Context("when multicasting a single dealing unit to empty posets", func() {

			BeforeEach(func() {
				rs = []gomel.RandomSource{}
				dags = []*dag{}

				tdag, _ := tests.CreateDagFromTestFile("../../testdata/one_unit4.txt", tests.NewTestDagFactory())
				rs = append(rs, tests.NewTestRandomSource())
				rs[0].Init(tdag)
				dags = append(dags, &dag{Dag: tdag.(*tests.Dag)})
				theUnit = tdag.MaximalUnitsPerProcess().Get(0)[0]

				for i := 1; i < 4; i++ {
					tdag, _ = tests.CreateDagFromTestFile("../../testdata/empty4.txt", tests.NewTestDagFactory())
					rs = append(rs, tests.NewTestRandomSource())
					rs[i].Init(tdag)
					dags = append(dags, &dag{Dag: tdag.(*tests.Dag)})
				}
			})

			It("should add the unit to empty copies", func() {
				servs[0].Send(theUnit)
				time.Sleep(time.Millisecond * 500)
				for i := 0; i < 4; i++ {
					servs[i].StopOut()
				}
				tests.CloseNetwork(netservs)
				for i := 0; i < 4; i++ {
					servs[i].StopIn()
				}
				Expect(dags[0].attemptedAdd).To(BeEmpty())
				for i := 1; i < 4; i++ {
					Expect(dags[i].attemptedAdd).To(HaveLen(1))
					Expect(dags[i].attemptedAdd[0].Parents()).To(HaveLen(0))
					Expect(dags[i].attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
					Expect(dags[i].attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				}
			})

		})

	})

})
