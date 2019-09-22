package fetch_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
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

type mockFB struct {
	happened bool
}

func (s *mockFB) Start()                       {}
func (s *mockFB) StopIn()                      {}
func (s *mockFB) StopOut()                     {}
func (s *mockFB) SetFallback(sync.QueryServer) {}
func (s *mockFB) FindOut(gomel.Preunit) {
	s.happened = true
}

var _ = Describe("Protocol", func() {

	var (
		dag1     gomel.Dag
		dag2     gomel.Dag
		adder1   *adder
		adder2   *adder
		serv1    sync.QueryServer
		serv2    sync.QueryServer
		fb       *mockFB
		netservs []network.Server
		pu       gomel.Preunit
		unit     gomel.Unit
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(10)
	})

	JustBeforeEach(func() {
		adder1 = &adder{tests.NewAdder(dag1), nil}
		adder2 = &adder{tests.NewAdder(dag2), nil}
		serv1 = NewServer(0, dag1, adder1, netservs[0], time.Second, zerolog.Nop(), 1, 0)
		serv2 = NewServer(1, dag2, adder2, netservs[1], time.Second, zerolog.Nop(), 0, 1)
		fb = &mockFB{}
		serv1.SetFallback(fb)
		serv1.Start()
		serv2.Start()

	})

	Describe("with only two participants", func() {

		Context("when requesting a nonexistent unit", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
			})

			It("should not add anything", func() {
				pu = creating.NewPreunit(0, nil, nil, nil)
				serv1.FindOut(pu)

				time.Sleep(time.Millisecond * 500)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()

				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(fb.happened).To(BeFalse())
			})

		})

		Context("when requesting a dealing unit", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				maxes := dag2.MaximalUnitsPerProcess()
				unit = maxes.Get(0)[0]
				pu = creating.NewPreunit(1, []*gomel.Hash{unit.Hash()}, nil, nil)

			})

			It("should add that unit", func() {
				serv1.FindOut(pu)

				time.Sleep(time.Millisecond * 500)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()

				Expect(adder1.attemptedAdd).To(HaveLen(1))
				Expect(adder1.attemptedAdd[0].Creator()).To(Equal(unit.Creator()))
				Expect(adder1.attemptedAdd[0].Signature()).To(Equal(unit.Signature()))
				Expect(adder1.attemptedAdd[0].Data()).To(Equal(unit.Data()))
				Expect(adder1.attemptedAdd[0].RandomSourceData()).To(Equal(unit.RandomSourceData()))
				Expect(adder1.attemptedAdd[0].Hash()).To(Equal(unit.Hash()))
				Expect(fb.happened).To(BeFalse())
			})

		})

		Context("when requesting a unit with unknown parents", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/random_10p_100u_2par.txt", tests.NewTestDagFactory())
				maxes := dag2.MaximalUnitsPerProcess()
				// Pick the hash of any maximal unit.
				maxes.Iterate(func(units []gomel.Unit) bool {
					for _, u := range units {
						unit = u
						return false
					}
					return true
				})
				pu = creating.NewPreunit(1, []*gomel.Hash{unit.Hash()}, nil, nil)
			})

			It("should fall back", func() {
				serv1.FindOut(pu)

				time.Sleep(time.Millisecond * 500)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()

				Expect(adder1.attemptedAdd).To(HaveLen(1))
				Expect(adder1.attemptedAdd[0].Creator()).To(Equal(unit.Creator()))
				Expect(adder1.attemptedAdd[0].Signature()).To(Equal(unit.Signature()))
				Expect(adder1.attemptedAdd[0].Data()).To(Equal(unit.Data()))
				Expect(adder1.attemptedAdd[0].RandomSourceData()).To(Equal(unit.RandomSourceData()))
				Expect(adder1.attemptedAdd[0].Hash()).To(Equal(unit.Hash()))
				Expect(fb.happened).To(BeTrue())
			})

		})

	})

})
