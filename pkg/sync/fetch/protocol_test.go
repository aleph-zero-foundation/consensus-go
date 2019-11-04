package fetch_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
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

func (s *mockFB) Resolve(gomel.Preunit) {
	s.happened = true
}

var _ = Describe("Protocol", func() {

	var (
		dag1     gomel.Dag
		dag2     gomel.Dag
		adder1   *adder
		adder2   *adder
		serv1    sync.Server
		serv2    sync.Server
		fbk1     sync.Fallback
		fb       *mockFB
		netservs []network.Server
		pu       gomel.Preunit
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(10)
	})

	JustBeforeEach(func() {
		adder1 = &adder{tests.NewAdder(dag1), nil}
		adder2 = &adder{tests.NewAdder(dag2), nil}
		serv1, fbk1 = NewServer(0, dag1, adder1, nil, netservs[0], time.Second, zerolog.Nop(), 1, 0)
		serv2, _ = NewServer(1, dag2, adder2, nil, netservs[1], time.Second, zerolog.Nop(), 0, 1)
		fb = &mockFB{}
		serv1.SetFallback(fb)
		serv1.Start()
		serv2.Start()

	})

	Describe("with only two participants", func() {

		Context("when requesting a unit with no parents", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
			})

			It("should not add anything", func() {
				pu = creating.NewPreunit(1, gomel.EmptyCrown(10), nil, nil)
				fbk1.Resolve(pu) // this is just a roundabout way to send a request to serv1

				time.Sleep(time.Millisecond * 500)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()

				Expect(adder1.attemptedAdd).To(BeEmpty())
				Expect(fb.happened).To(BeFalse())
			})

		})

		Context("when requesting a unit with unknown parents", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/random_100u.txt", tests.NewTestDagFactory())
				unit := dag2.MaximalUnitsPerProcess().Get(1)[0]
				enc, err := encoding.EncodeUnit(unit)
				Expect(err).NotTo(HaveOccurred())
				pu, err = encoding.DecodePreunit(enc)
				Expect(err).NotTo(HaveOccurred())
				Expect(adder1.AddUnit(pu)).ToNot(Succeed())
			})

			It("should add enough units to add the preunit", func() {
				fbk1.Resolve(pu)

				time.Sleep(time.Millisecond * 500)
				serv1.StopOut()
				tests.CloseNetwork(netservs)
				serv2.StopIn()

				Expect(fb.happened).To(BeFalse())
				Expect(adder1.AddUnit(pu)).To(Succeed())
			})
		})

	})

})
