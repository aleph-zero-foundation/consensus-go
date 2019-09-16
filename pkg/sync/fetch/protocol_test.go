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

type dag struct {
	*tests.Dag
	attemptedAdd []gomel.Preunit
}

func (dag *dag) AddUnit(unit gomel.Preunit, rs gomel.RandomSource, callback gomel.Callback) {
	dag.attemptedAdd = append(dag.attemptedAdd, unit)
	dag.Dag.AddUnit(unit, rs, callback)
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
		dag1     *dag
		dag2     *dag
		rs1      gomel.RandomSource
		rs2      gomel.RandomSource
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
		serv1 = NewServer(0, dag1, rs1, netservs[0], time.Second, zerolog.Nop(), 1, 0)
		serv2 = NewServer(1, dag2, rs2, netservs[1], time.Second, zerolog.Nop(), 0, 1)
		fb = &mockFB{}
		serv1.SetFallback(fb)
		serv1.Start()
		serv2.Start()

	})

	Describe("with only two participants", func() {

		Context("when requesting a nonexistent unit", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
			})

			It("should not add anything", func() {
				pu = creating.NewPreunit(0, nil, nil, nil)
				serv1.FindOut(pu)

				time.Sleep(time.Millisecond * 500)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()

				Expect(dag1.attemptedAdd).To(BeEmpty())
				Expect(fb.happened).To(BeFalse())
			})

		})

		Context("when requesting a dealing unit", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				rs1 = tests.NewTestRandomSource()
				rs1.Init(tdag1)
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
				rs2 = tests.NewTestRandomSource()
				rs2.Init(tdag2)
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
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

				Expect(dag1.attemptedAdd).To(HaveLen(1))
				Expect(dag1.attemptedAdd[0].Creator()).To(Equal(unit.Creator()))
				Expect(dag1.attemptedAdd[0].Signature()).To(Equal(unit.Signature()))
				Expect(dag1.attemptedAdd[0].Data()).To(Equal(unit.Data()))
				Expect(dag1.attemptedAdd[0].RandomSourceData()).To(Equal(unit.RandomSourceData()))
				Expect(dag1.attemptedAdd[0].Hash()).To(Equal(unit.Hash()))
				Expect(fb.happened).To(BeFalse())
			})

		})

		Context("when requesting a unit with unknown parents", func() {

			BeforeEach(func() {
				tdag1, _ := tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				dag1 = &dag{Dag: tdag1.(*tests.Dag)}
				tdag2, _ := tests.CreateDagFromTestFile("../../testdata/random_10p_100u_2par.txt", tests.NewTestDagFactory())
				dag2 = &dag{Dag: tdag2.(*tests.Dag)}
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

				Expect(dag1.attemptedAdd).To(HaveLen(1))
				Expect(dag1.attemptedAdd[0].Creator()).To(Equal(unit.Creator()))
				Expect(dag1.attemptedAdd[0].Signature()).To(Equal(unit.Signature()))
				Expect(dag1.attemptedAdd[0].Data()).To(Equal(unit.Data()))
				Expect(dag1.attemptedAdd[0].RandomSourceData()).To(Equal(unit.RandomSourceData()))
				Expect(dag1.attemptedAdd[0].Hash()).To(Equal(unit.Hash()))
				Expect(fb.happened).To(BeTrue())
			})

		})

	})

})
