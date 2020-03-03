package fetch_test

import (
	snc "sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/network"
	ctests "gitlab.com/alephledger/core-go/pkg/tests"
)

type testServer interface {
	In()
	Out()
}

type unitsAdder struct {
	gomel.Orderer
	gomel.Adder
	mx           snc.Mutex
	attemptedAdd []gomel.Preunit
	dag          gomel.Dag
}

func (ua *unitsAdder) AddPreunits(source uint16, units ...gomel.Preunit) []error {
	ua.mx.Lock()
	ua.attemptedAdd = append(ua.attemptedAdd, units...)
	ua.mx.Unlock()
	err := ua.Adder.AddPreunits(source, units...)
	if err != nil {
		return err
	}
	return nil
}

func (ua *unitsAdder) UnitsByID(ids ...uint64) []gomel.Unit {
	var result []gomel.Unit
	for _, id := range ids {
		_, _, epoch := gomel.DecodeID(id)
		if epoch == ua.dag.EpochID() {
			result = append(result, ua.dag.GetByID(id)...)
		}
	}
	return result
}

// missingParents returns a slice of unit IDs that are parents of preunit above maxUnits.
func missingParents(preunit gomel.Preunit, maxUnits gomel.SlottedUnits) []uint64 {
	unitIDs := []uint64{}
	requiredHeights := preunit.View().Heights
	curCreator := uint16(0)
	maxUnits.Iterate(func(units []gomel.Unit) bool {
		highest := -1
		for _, u := range units {
			if u.Height() > highest {
				highest = u.Height()
			}
		}
		highest++
		for highest <= requiredHeights[curCreator] {
			unitIDs = append(unitIDs, gomel.ID(highest, curCreator, preunit.EpochID()))
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
		adder1   *unitsAdder
		adder2   gomel.Orderer
		serv1    sync.Server
		serv2    sync.Server
		request  sync.Fetch
		tserv1   testServer
		tserv2   testServer
		netservs []network.Server
		pu       gomel.Preunit
		missing  []uint64
	)

	BeforeEach(func() {
		netservs = ctests.NewNetwork(10)
	})

	JustBeforeEach(func() {
		config1 := config.Empty()
		config1.NProc = 2
		config1.Pid = 0
		config1.Timeout = time.Second
		if adder1 == nil {
			panic("adder1 is nil")
		}
		serv1, request = NewServer(config1, adder1, netservs[0], zerolog.Nop())
		config2 := config.Empty()
		config2.NProc = 2
		config2.Pid = 1
		config2.Timeout = time.Second
		serv2, _ = NewServer(config2, adder2, netservs[1], zerolog.Nop())
		tserv1 = serv1.(testServer)
		tserv2 = serv2.(testServer)
	})

	JustAfterEach(func() {
		ctests.CloseNetwork(netservs)
	})

	Describe("with only two participants", func() {

		Context("when requesting a unit with unknown parents", func() {

			BeforeEach(func() {
				dag1, _, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				adder1 = &unitsAdder{dag: dag1, Orderer: tests.NewOrderer(), Adder: tests.NewAdder(dag1)}
				var testAdder gomel.Adder
				dag2, testAdder, _ = tests.CreateDagFromTestFile("../../testdata/dags/10/random_100u.txt", tests.NewTestDagFactory())
				adder2 = &unitsAdder{dag: dag2, Orderer: tests.NewOrderer(), Adder: testAdder}
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
