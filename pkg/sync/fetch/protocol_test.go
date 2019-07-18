package fetch_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type dag struct {
	*tests.Dag
	attemptedAdd []gomel.Preunit
}

func (p *dag) AddUnit(unit gomel.Preunit, rs gomel.RandomSource, callback func(gomel.Preunit, gomel.Unit, error)) {
	p.attemptedAdd = append(p.attemptedAdd, unit)
	p.Dag.AddUnit(unit, rs, callback)
}

type fallback bool

func (f *fallback) Run(_ gomel.Preunit) {
	*f = true
}

var _ = Describe("Protocol", func() {

	var (
		p1         *dag
		p2         *dag
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
		proto1 = NewProtocol(0, p1, tests.NewTestRandomSource(p1), reqs, d, ls[0], time.Second, &fallenBack, make(chan int), zerolog.Nop())
		proto2 = NewProtocol(1, p2, tests.NewTestRandomSource(p2), reqs, d, ls[1], time.Second, &fallenBack, make(chan int), zerolog.Nop())
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
				tp, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				p1 = &dag{
					Dag:          tp.(*tests.Dag),
					attemptedAdd: nil,
				}
				p2 = p1
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
				Expect(p1.attemptedAdd).To(BeEmpty())
				Expect(bool(fallenBack)).To(BeFalse())
			})

		})

		Context("when requesting a dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				tp, _ := tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				p1 = &dag{
					Dag:          tp.(*tests.Dag),
					attemptedAdd: nil,
				}
				p2 = p1
				maxes := p1.MaximalUnitsPerProcess()
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
				Expect(p1.attemptedAdd).To(HaveLen(1))
				Expect(p1.attemptedAdd[0].Creator()).To(Equal(theUnit.Creator()))
				Expect(p1.attemptedAdd[0].Signature()).To(Equal(theUnit.Signature()))
				Expect(p1.attemptedAdd[0].Data()).To(Equal(theUnit.Data()))
				Expect(p1.attemptedAdd[0].RandomSourceData()).To(Equal(theUnit.RandomSourceData()))
				Expect(p1.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				Expect(bool(fallenBack)).To(BeFalse())
			})

		})

		Context("when requesting a unit with unknown parents", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				tp1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				p1 = &dag{
					Dag:          tp1.(*tests.Dag),
					attemptedAdd: nil,
				}
				tp2, _ := tests.CreateDagFromTestFile("../../testdata/random_10p_100u_2par.txt", tests.NewTestDagFactory())
				p2 = &dag{
					Dag:          tp2.(*tests.Dag),
					attemptedAdd: nil,
				}
				maxes := p2.MaximalUnitsPerProcess()
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
				Expect(p1.attemptedAdd).To(HaveLen(1))
				Expect(p1.attemptedAdd[0].Creator()).To(Equal(theUnit.Creator()))
				Expect(p1.attemptedAdd[0].Signature()).To(Equal(theUnit.Signature()))
				Expect(p1.attemptedAdd[0].Data()).To(Equal(theUnit.Data()))
				Expect(p1.attemptedAdd[0].RandomSourceData()).To(Equal(theUnit.RandomSourceData()))
				Expect(p1.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				Expect(bool(fallenBack)).To(BeTrue())
			})

		})

	})

})
