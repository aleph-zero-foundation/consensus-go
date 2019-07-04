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

type poset struct {
	*tests.Poset
	attemptedAdd []gomel.Preunit
}

func (p *poset) AddUnit(unit gomel.Preunit, callback func(gomel.Preunit, gomel.Unit, error)) {
	p.attemptedAdd = append(p.attemptedAdd, unit)
	p.Poset.AddUnit(unit, callback)
}

var _ = Describe("Protocol", func() {

	var (
		p1         *poset
		p2         *poset
		reqs       chan Request
		fallenBack bool
		proto1     gsync.Protocol
		proto2     gsync.Protocol
		d          network.Dialer
		ls         []network.Listener
	)

	Fallback := func(_ gomel.Preunit) {
		fallenBack = true
	}

	BeforeEach(func() {
		d, ls = tests.NewNetwork(10)
		fallenBack = false
		reqs = make(chan Request)
	})

	JustBeforeEach(func() {
		proto1 = NewProtocol(0, p1, tests.NewTestRandomSource(p1), reqs, d, ls[0], time.Second, Fallback, make(chan int), zerolog.Logger{})
		proto2 = NewProtocol(1, p2, tests.NewTestRandomSource(p2), reqs, d, ls[1], time.Second, Fallback, make(chan int), zerolog.Logger{})
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
				tp, _ := tests.CreatePosetFromTestFile("../../testdata/empty.txt", tests.NewTestPosetFactory())
				p1 = &poset{
					Poset:        tp.(*tests.Poset),
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
				Expect(fallenBack).To(BeFalse())
			})

		})

		Context("when requesting a dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				tp, _ := tests.CreatePosetFromTestFile("../../testdata/one_unit.txt", tests.NewTestPosetFactory())
				p1 = &poset{
					Poset:        tp.(*tests.Poset),
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
				Expect(fallenBack).To(BeFalse())
			})

		})

		Context("when requesting a unit with unknown parents", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				// TODO: actually need two posets here, so fix that somehow
				tp1, _ := tests.CreatePosetFromTestFile("../../testdata/empty.txt", tests.NewTestPosetFactory())
				p1 = &poset{
					Poset:        tp1.(*tests.Poset),
					attemptedAdd: nil,
				}
				tp2, _ := tests.CreatePosetFromTestFile("../../testdata/random_10p_100u_2par.txt", tests.NewTestPosetFactory())
				p2 = &poset{
					Poset:        tp2.(*tests.Poset),
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
				Expect(fallenBack).To(BeTrue())
			})

		})

	})
	// TODO: More tests and fix what is already here.

})
