package multicast_test

import (
	snc "sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/network"
	ctests "gitlab.com/alephledger/core-go/pkg/tests"
)

type testServer interface {
	In()
	Out(uint16)
}

type unitsAdder struct {
	gomel.Orderer
	gomel.Adder
	attemptedAdd []gomel.Preunit
	mx           snc.Mutex
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

var _ = Describe("Protocol", func() {

	var (
		dags      []gomel.Dag
		adders    []*unitsAdder
		servs     []sync.Server
		tservs    []testServer
		netservs  []network.Server
		multicast sync.Multicast
		pu        gomel.Preunit
	)

	BeforeEach(func() {
		netservs = ctests.NewNetwork(4)
	})

	AfterEach(func() {
		ctests.CloseNetwork(netservs)
	})

	JustBeforeEach(func() {
		adders = nil
		for _, dag := range dags {
			adders = append(adders, &unitsAdder{Orderer: tests.NewOrderer(), Adder: tests.NewAdder(dag)})
		}
		for i := 0; i < 4; i++ {
			config := config.Empty()
			config.NProc = 4
			config.Pid = uint16(i)
			serv, mltcst := NewServer(config, adders[i], netservs[i], 10*time.Second, zerolog.Nop())
			servs = append(servs, serv)
			tservs = append(tservs, serv.(testServer))
			if multicast == nil {
				multicast = mltcst
			}
		}
	})

	Describe("in a small dag", func() {

		Context("when multicasting a single dealing unit to empty dags", func() {

			BeforeEach(func() {
				dags = []gomel.Dag{}
				for i := 0; i < 4; i++ {
					dag, _, _ := tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
					dags = append(dags, dag)
				}
				pu = tests.NewPreunit(uint16(0), gomel.EmptyCrown(4), []byte{}, []byte{}, nil)

			})

			It("should add the unit to empty copies", func() {
				adders[0].AddPreunits(0, pu)
				unit := dags[0].MaximalUnitsPerProcess().Get(0)[0]
				var wg snc.WaitGroup
				for i := uint16(1); i < 4; i++ {
					wg.Add(1)
					go func(pid uint16) {
						defer wg.Done()
						tservs[0].Out(pid)
					}(i)
				}
				multicast(unit)
				for i := 1; i < 4; i++ {
					tservs[i].In()
				}
				wg.Wait()
				for i := 0; i < 4; i++ {
					Expect(adders[i].attemptedAdd).To(HaveLen(1))
					Expect(adders[i].attemptedAdd[0].Creator()).To(BeNumerically("==", 0))
					Expect(adders[i].attemptedAdd[0].Hash()).To(Equal(pu.Hash()))
				}
			})
		})
	})
})
