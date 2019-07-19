package fetch_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
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

type fallback bool

func (f *fallback) Run(_ gomel.Preunit) {
	*f = true
}

var _ = Describe("Protocol", func() {

	var (
		dag1       *dag
		dag2       *dag
		reqs       chan Request
		fallenBack fallback
		proto1     gsync.Protocol
		proto2     gsync.Protocol
		d          network.Dialer
		ls         []network.Listener
	)

	BeforeEach(func() {
		d, ls = tests.NewNetwork(10)
		fallenBack = false
		reqs = make(chan Request)
	})

	JustBeforeEach(func() {
		trs1 := tests.NewTestRandomSource()
		trs1.Init(dag1)
		proto1 = NewProtocol(0, dag1, trs1, reqs, d, ls[0], time.Second, &fallenBack, make(chan int), zerolog.Nop())
		trs2 := tests.NewTestRandomSource()
		trs2.Init(dag2)
		proto2 = NewProtocol(1, dag2, trs2, reqs, d, ls[1], time.Second, &fallenBack, make(chan int), zerolog.Nop())
	})

	Describe("with only two participants", func() {

		var (
			callee uint16
		)

		BeforeEach(func() {
			callee = 1
		})

		Context("when requesting a nonexistent unit", func() {

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
					proto2.In()
					wg.Done()
				}()
				go func() {
					proto1.Out()
					wg.Done()
				}()
				req := Request{
					Pid:    callee,
					Hashes: []*gomel.Hash{&gomel.Hash{}},
				}
				req.Hashes[0][0] = 1
				reqs <- req
				wg.Wait()
				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(bool(fallenBack)).To(BeFalse())
			})

		})

		Context("when requesting a dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				td, _ := tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				dag1 = &dag{
					Dag:          td.(*tests.Dag),
					attemptedAdd: nil,
				}
				dag2 = dag1
				maxes := dag1.MaximalUnitsPerProcess()
				// Pick the hash of the only unit.
				maxes.Iterate(func(units []gomel.Unit) bool {
					for _, u := range units {
						theUnit = u
						return false
					}
					return true
				})
			})

			It("should add that unit", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto2.In()
					wg.Done()
				}()
				go func() {
					proto1.Out()
					wg.Done()
				}()
				req := Request{
					Pid:    callee,
					Hashes: []*gomel.Hash{theUnit.Hash()},
				}
				reqs <- req
				wg.Wait()
				Expect(dag1.attemptedAdd).To(HaveLen(1))
				Expect(dag1.attemptedAdd[0].Creator()).To(Equal(theUnit.Creator()))
				Expect(dag1.attemptedAdd[0].Signature()).To(Equal(theUnit.Signature()))
				Expect(dag1.attemptedAdd[0].Data()).To(Equal(theUnit.Data()))
				Expect(dag1.attemptedAdd[0].RandomSourceData()).To(Equal(theUnit.RandomSourceData()))
				Expect(dag1.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				Expect(bool(fallenBack)).To(BeFalse())
			})

		})

		Context("when requesting a unit with unknown parents", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag1 = &dag{
					Dag:          tdag1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/random_10p_100u_2par.txt", tests.NewTestDagFactory())
				dag2 = &dag{
					Dag:          tdag2.(*tests.Dag),
					attemptedAdd: nil,
				}
				maxes := dag2.MaximalUnitsPerProcess()
				// Pick the hash of any maximal unit.
				maxes.Iterate(func(units []gomel.Unit) bool {
					for _, u := range units {
						theUnit = u
						return false
					}
					return true
				})
			})

			It("should fall back", func() {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					proto2.In()
					wg.Done()
				}()
				go func() {
					proto1.Out()
					wg.Done()
				}()
				req := Request{
					Pid:    callee,
					Hashes: []*gomel.Hash{theUnit.Hash()},
				}
				reqs <- req
				wg.Wait()
				Expect(dag1.attemptedAdd).To(HaveLen(1))
				Expect(dag1.attemptedAdd[0].Creator()).To(Equal(theUnit.Creator()))
				Expect(dag1.attemptedAdd[0].Signature()).To(Equal(theUnit.Signature()))
				Expect(dag1.attemptedAdd[0].Data()).To(Equal(theUnit.Data()))
				Expect(dag1.attemptedAdd[0].RandomSourceData()).To(Equal(theUnit.RandomSourceData()))
				Expect(dag1.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				Expect(bool(fallenBack)).To(BeTrue())
			})

		})

	})

})
