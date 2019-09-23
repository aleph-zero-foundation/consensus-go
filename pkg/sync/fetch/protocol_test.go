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

type fallback bool

func (f *fallback) Run(_ gomel.Preunit) {
	*f = true
}

func (f *fallback) Stop() {}

var _ = Describe("Protocol", func() {

	var (
		dag1       gomel.Dag
		adder1     *adder
		dag2       gomel.Dag
		adder2     *adder
		reqs       chan Request
		fallenBack fallback
		proto1     gsync.Protocol
		proto2     gsync.Protocol
		servs      []network.Server
	)

	BeforeEach(func() {
		servs = tests.NewNetwork(10)
		fallenBack = false
		reqs = make(chan Request)
	})

	JustBeforeEach(func() {
		adder1 = &adder{tests.NewAdder(dag1), nil}
		adder2 = &adder{tests.NewAdder(dag2), nil}
		proto1 = NewProtocol(0, dag1, adder1, reqs, servs[0], time.Second, &fallenBack, zerolog.Nop())
		proto2 = NewProtocol(1, dag2, adder2, reqs, servs[1], time.Second, &fallenBack, zerolog.Nop())
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
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
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
				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(bool(fallenBack)).To(BeFalse())
			})

		})

		Context("when requesting a dealing unit", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/one_unit.txt", tests.NewTestDagFactory())
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
				Expect(adder1.attemptedAdd).To(HaveLen(1))
				Expect(adder1.attemptedAdd[0].Creator()).To(Equal(theUnit.Creator()))
				Expect(adder1.attemptedAdd[0].Signature()).To(Equal(theUnit.Signature()))
				Expect(adder1.attemptedAdd[0].Data()).To(Equal(theUnit.Data()))
				Expect(adder1.attemptedAdd[0].RandomSourceData()).To(Equal(theUnit.RandomSourceData()))
				Expect(adder1.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				Expect(bool(fallenBack)).To(BeFalse())
			})

		})

		Context("when requesting a unit with unknown parents", func() {

			var (
				theUnit gomel.Unit
			)

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/random_100u_2par.txt", tests.NewTestDagFactory())
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
				Expect(adder1.attemptedAdd).To(HaveLen(1))
				Expect(adder1.attemptedAdd[0].Creator()).To(Equal(theUnit.Creator()))
				Expect(adder1.attemptedAdd[0].Signature()).To(Equal(theUnit.Signature()))
				Expect(adder1.attemptedAdd[0].Data()).To(Equal(theUnit.Data()))
				Expect(adder1.attemptedAdd[0].RandomSourceData()).To(Equal(theUnit.RandomSourceData()))
				Expect(adder1.attemptedAdd[0].Hash()).To(Equal(theUnit.Hash()))
				Expect(bool(fallenBack)).To(BeTrue())
			})

		})

	})

})
