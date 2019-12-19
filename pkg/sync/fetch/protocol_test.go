package fetch_test

import (
	snc "sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/validator-skeleton/pkg/network"
)

type testServer interface {
	In()
	Out()
}

type adder struct {
	gomel.Adder
	mx           snc.Mutex
	attemptedAdd []gomel.Preunit
}

func (a *adder) AddUnit(unit gomel.Preunit, source uint16) error {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, unit)
	a.mx.Unlock()
	return a.Adder.AddUnit(unit, source)
}

func (a *adder) AddUnits(units []gomel.Preunit, source uint16) *gomel.AggregateError {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, units...)
	a.mx.Unlock()
	return a.Adder.AddUnits(units, source)
}

// missingParents returns a slice of unit IDs that are parents of preunit above maxUnits.
func missingParents(preunit gomel.Preunit, maxUnits gomel.SlottedUnits) []uint64 {
	unitIDs := []uint64{}
	requiredHeights := preunit.View().Heights
	curCreator := uint16(0)
	nProc := uint16(len(requiredHeights))
	maxUnits.Iterate(func(units []gomel.Unit) bool {
		highest := -1
		for _, u := range units {
			if u.Height() > highest {
				highest = u.Height()
			}
		}
		highest++
		for highest <= requiredHeights[curCreator] {
			unitIDs = append(unitIDs, gomel.ID(highest, curCreator, nProc))
			highest++
		}
		curCreator++
		return true
	})
	return unitIDs
}

var _ = Describe("Protocol", func() {

	var (
		dag1     gomel.Dag
		dag2     gomel.Dag
		adder1   *adder
		adder2   gomel.Adder
		serv1    sync.Server
		serv2    sync.Server
		request  gomel.RequestFetch
		tserv1   testServer
		tserv2   testServer
		netservs []network.Server
		pu       gomel.Preunit
		missing  []uint64
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(10)
	})

	JustBeforeEach(func() {
		serv1, request = NewServer(0, dag1, adder1, netservs[0], time.Second, zerolog.Nop(), 0, 0)
		serv2, _ = NewServer(1, dag2, adder2, netservs[1], time.Second, zerolog.Nop(), 0, 0)
		tserv1 = serv1.(testServer)
		tserv2 = serv2.(testServer)
	})

	JustAfterEach(func() {
		tests.CloseNetwork(netservs)
	})

	Describe("with only two participants", func() {

		Context("when requesting a unit with unknown parents", func() {

			BeforeEach(func() {
				dag1, _, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				adder1 = &adder{Adder: tests.NewAdder(dag1)}
				dag2, adder2, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/random_100u.txt", tests.NewTestDagFactory())
				max1 := dag1.MaximalUnitsPerProcess()
				unit := dag2.MaximalUnitsPerProcess().Get(1)[0]
				enc, err := encoding.EncodeUnit(unit)
				Expect(err).NotTo(HaveOccurred())
				pu, err = encoding.DecodePreunit(enc)
				Expect(err).NotTo(HaveOccurred())
				missing = missingParents(pu, max1)
			})

			It("should add enough units to add the preunit", func() {
				request(pu.Creator(), missing)
				go tserv2.In()
				tserv1.Out()
				Expect(adder1.attemptedAdd).To(HaveLen(len(missing)))
			})
		})

	})

})
