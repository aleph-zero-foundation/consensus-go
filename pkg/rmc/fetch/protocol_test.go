package fetch_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	. "gitlab.com/alephledger/consensus-go/pkg/rmc/fetch"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
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

func preunitFromUnit(u gomel.Unit) gomel.Preunit {
	parents := []*gomel.Hash{}
	for _, p := range u.Parents() {
		parents = append(parents, p.Hash())
	}
	return tests.NewPreunit(u.Creator(), parents, u.Data(), u.RandomSourceData())
}

var _ = Describe("Protocol", func() {

	var (
		dag1   *dag
		dag2   *dag
		reqs   chan gomel.Preunit
		proto1 gsync.Protocol
		proto2 gsync.Protocol
		servs  []network.Server
	)

	BeforeEach(func() {
		servs = tests.NewNetwork(10)
		reqs = make(chan gomel.Preunit)
	})

	JustBeforeEach(func() {
		trs1 := tests.NewTestRandomSource()
		trs1.Init(dag1)
		proto1 = NewProtocol(0, dag1, trs1, nil, servs[0], time.Second, zerolog.Nop())
		trs2 := tests.NewTestRandomSource()
		trs2.Init(dag2)
		proto2 = NewProtocol(1, dag2, trs2, reqs, servs[1], time.Second, zerolog.Nop())
	})

	Describe("with only two participants", func() {

		var (
			callee uint16
		)

		BeforeEach(func() {
			callee = 1
		})

		Context("when requesting a nonexistent preunit", func() {

			BeforeEach(func() {
				td, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag1 = &dag{
					Dag:          td.(*tests.Dag),
					attemptedAdd: nil,
				}
				dag2 = dag1
			})

			It("should not add anything", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto2.Out()
					wg.Done()
				}()
				go func() {
					proto1.In()
					wg.Done()
				}()
				reqs <- tests.NewPreunit(0, nil, nil, nil)
				wg.Wait()
				Expect(dag1.attemptedAdd).To(BeEmpty())
			})

		})

		Context("when requesting a single unit with two parents", func() {
			BeforeEach(func() {
				td1, _ := tests.CreateDagFromTestFile("../../testdata/single_unit_with_two_parents.txt", tests.NewTestDagFactory())
				td2, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag1 = &dag{
					Dag:          td1.(*tests.Dag),
					attemptedAdd: nil,
				}
				dag2 = &dag{
					Dag:          td2.(*tests.Dag),
					attemptedAdd: nil,
				}
			})

			It("should add all the units", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto2.Out()
					wg.Done()
				}()
				go func() {
					proto1.In()
					wg.Done()
				}()
				pu := dag1.MaximalUnitsPerProcess().Get(0)[0]
				reqs <- preunitFromUnit(pu)
				wg.Wait()
				Expect(dag2.attemptedAdd).To(HaveLen(3))
				Expect(dag2.attemptedAdd[2].Hash()).To(Equal(pu.Hash()))
			})

		})
	})

})
