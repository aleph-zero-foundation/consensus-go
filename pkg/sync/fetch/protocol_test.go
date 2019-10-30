package fetch_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type testServer interface {
	In()
	Out()
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
		adder1   gomel.Adder
		adder2   gomel.Adder
		serv1    sync.Server
		serv2    sync.Server
		tserv1   testServer
		tserv2   testServer
		fbk1     sync.Fallback
		fb       *mockFB
		netservs []network.Server
		pu       gomel.Preunit
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(10)
	})

	JustBeforeEach(func() {
		adder1 = tests.NewAdder(dag1)
		adder2 = tests.NewAdder(dag2)
		serv1, fbk1 = NewServer(0, dag1, adder1, nil, netservs[0], time.Second, zerolog.Nop(), 0, 0)
		serv2, _ = NewServer(1, dag2, adder2, nil, netservs[1], time.Second, zerolog.Nop(), 0, 0)
		tserv1 = serv1.(testServer)
		tserv2 = serv2.(testServer)
		fb = &mockFB{}
		serv1.SetFallback(fb)
	})

	JustAfterEach(func() {
		tests.CloseNetwork(netservs)
	})

	Describe("with only two participants", func() {

		Context("when requesting a unit with unknown parents", func() {

			BeforeEach(func() {
				dag1, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				dag2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/random_100u.txt", tests.NewTestDagFactory())
				unit := dag2.MaximalUnitsPerProcess().Get(1)[0]
				enc, err := encoding.EncodeUnit(unit)
				Expect(err).NotTo(HaveOccurred())
				pu, err = encoding.DecodePreunit(enc)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add enough units to add the preunit", func() {
				Expect(adder1.AddUnit(pu)).ToNot(Succeed())
				fbk1.Resolve(pu)

				go tserv2.In()
				tserv1.Out()

				Expect(fb.happened).To(BeFalse())
				Expect(adder1.AddUnit(pu)).To(Succeed())
			})
		})

	})

})
