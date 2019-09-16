package retrying_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/retrying"
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

func pre(u gomel.Unit) gomel.Preunit {
	parents := u.Parents()
	hashes := make([]*gomel.Hash, len(parents))
	for i := 0; i < len(parents); i++ {
		hashes[i] = parents[i].Hash()
	}
	pu := creating.NewPreunit(u.Creator(), hashes, u.Data(), u.RandomSourceData())
	pu.SetSignature(u.Signature())
	return pu
}

var _ = Describe("Protocol", func() {

	var (
		dag1     *dag
		dag2     *dag
		rs1      gomel.RandomSource
		rs2      gomel.RandomSource
		serv1    sync.QueryServer
		serv2    sync.QueryServer
		retr     sync.QueryServer
		netservs []network.Server
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(10)
	})

	JustBeforeEach(func() {
		serv1 = fetch.NewServer(0, dag1, rs1, netservs[0], time.Second, zerolog.Nop(), 1, 0)
		serv2 = fetch.NewServer(1, dag2, rs2, netservs[1], time.Second, zerolog.Nop(), 0, 1)
		retr = NewServer(dag1, rs1, time.Millisecond*100, zerolog.Nop())
		retr.SetFallback(serv1)
		serv1.Start()
		serv2.Start()
		retr.Start()
	})

	Describe("with only two participants", func() {

		Context("when requesting a dealing unit", func() {

			var (
				unit gomel.Unit
				pu   gomel.Preunit
			)

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
				maxes := dag2.MaximalUnitsPerProcess()
				unit = maxes.Get(0)[0]
				pu = pre(unit)
			})

			It("should add it directly", func() {
				retr.FindOut(pu)

				time.Sleep(time.Millisecond * 500)
				retr.StopIn()
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()

				Expect(dag1.attemptedAdd).To(HaveLen(1))
				Expect(dag1.attemptedAdd[0].Creator()).To(Equal(unit.Creator()))
				Expect(dag1.attemptedAdd[0].Signature()).To(Equal(unit.Signature()))
				Expect(dag1.attemptedAdd[0].Data()).To(Equal(unit.Data()))
				Expect(dag1.attemptedAdd[0].RandomSourceData()).To(Equal(unit.RandomSourceData()))
				Expect(dag1.attemptedAdd[0].Hash()).To(Equal(unit.Hash()))
			})

		})

	})

})
