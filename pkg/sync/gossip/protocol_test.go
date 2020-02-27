package gossip_test

import (
	"bytes"
	"errors"
	"sort"
	snc "sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/network"
	ctests "gitlab.com/alephledger/core-go/pkg/tests"
)

func collectUnits(dag gomel.Dag) []gomel.Unit {
	units := tests.CollectUnits(dag)
	result := []gomel.Unit{}
	processed := map[gomel.Hash]bool{}
	for unit := range units {
		if processed[*unit.Hash()] {
			continue
		}
		result = append(result, unit)
		processed[*unit.Hash()] = true
	}
	sort.Slice(result, func(i, j int) bool {
		return bytes.Compare(result[i].Hash()[:], result[j].Hash()[:]) < 0
	})

	return result
}

type testServer interface {
	In()
	Out()
}

type unitsAdder struct {
	gomel.Orderer
	gomel.Adder
	dag gomel.Dag
}

func (ua *unitsAdder) AddPreunits(source uint16, units ...gomel.Preunit) []error {
	return ua.Adder.AddPreunits(source, units...)
}

func (ua *unitsAdder) GetInfo() [2]*gomel.DagInfo {
	return [2]*gomel.DagInfo{}
}

func (ua *unitsAdder) Delta(info [2]*gomel.DagInfo) []gomel.Unit {
	return collectUnits(ua.dag)
}

type testNetworkServer struct {
	network.Server
	connectivity []bool
}

func (ns testNetworkServer) Dial(k uint16, timeout time.Duration) (network.Connection, error) {
	if !ns.connectivity[k] {
		<-time.After(timeout)
		return nil, errors.New("unable to connect")
	}
	return ns.Server.Dial(k, timeout)
}

func newNetwork(length int, connectivity [][]bool) []network.Server {
	network := ctests.NewNetwork(length)
	for ix := range network {
		network[ix] = &testNetworkServer{Server: network[ix], connectivity: connectivity[ix]}
	}
	return network
}

var _ = Describe("Protocol", func() {

	var (
		dags     []gomel.Dag
		adders   []*unitsAdder
		servs    []sync.Server
		tservs   []testServer
		netservs []network.Server
	)

	BeforeEach(func() {
		dags = []gomel.Dag{}
	})

	AfterEach(func() {
		ctests.CloseNetwork(netservs)
	})

	Describe("in a small dag", func() {

		init := func(connectionTimeout time.Duration) {
			size := len(dags)
			adders = nil
			for _, dag := range dags {
				adder := &unitsAdder{Orderer: tests.NewOrderer(), Adder: tests.NewAdder(dag), dag: dag}
				adders = append(adders, adder)
			}
			servs = make([]sync.Server, size)
			tservs = make([]testServer, size)
			for i := 0; i < size; i++ {
				config := config.Empty()
				config.NProc = uint16(size)
				config.Pid = uint16(i)
				servs[i], _ = NewServer(config, adders[i], netservs[i], connectionTimeout, zerolog.Nop(), 1, 1, 2)
				tservs[i] = servs[i].(testServer)
			}
		}

		prepareRingConnectivity := func(nProc int) [][]bool {
			connectivity := make([][]bool, nProc)
			for ix := range connectivity {
				connectivity[ix] = make([]bool, nProc)
				connectivity[ix][(ix+1)%nProc] = true
				prev := ix - 1
				if ix == 0 {
					prev = nProc - 1
				}
				connectivity[ix][prev] = true
			}
			return connectivity
		}

		prepareSmallDags := func(nProc int) []gomel.Dag {
			dags := []gomel.Dag{}
			for ix := 0; ix < nProc; ix++ {
				dag, adder := tests.NewTestDagFactory().CreateDag(uint16(nProc))

				parents := make([]*gomel.Hash, nProc)
				parentsHeights := make([]int, nProc)
				for i := 0; i < nProc; i++ {
					parentsHeights[i] = -1
				}
				unitData := make([]byte, 4)
				pu := tests.NewPreunit(
					uint16(ix),
					gomel.NewCrown(parentsHeights, gomel.CombineHashes(parents)),
					unitData,
					[]byte{},
					nil,
				)

				err := adder.AddPreunits(uint16(ix), pu)
				Expect(err).To(BeEmpty())

				dags = append(dags, dag)
			}
			return dags
		}

		performTest := func(repetitions int) {
			var done snc.WaitGroup

			for i := 0; i < repetitions; i++ {
				done.Add(len(tservs))
				for _, serv := range tservs {
					go func(serv testServer) {
						defer done.Done()
						serv.In()
					}(serv)
				}
				done.Add(len(tservs))
				for _, serv := range tservs {
					go func(serv testServer) {
						defer done.Done()
						serv.Out()
					}(serv)
				}
				done.Wait()
			}
		}

		Context("when all dags contain different units and network is a circle without a single connection", func() {

			It("should after enough long time make all dags contain same units", func() {
				nProc := 3
				connectivity := prepareRingConnectivity(nProc)
				connectivity[0][2] = false
				connectivity[2][0] = false
				netservs = newNetwork(nProc, connectivity)
				dags = prepareSmallDags(nProc)
				init(10 * time.Millisecond)

				// expected number of rounds to cover whole network
				expectedTime := 4
				performTest(expectedTime * 5)

				allUnits := collectUnits(dags[0])
				for ix := range dags[1:] {
					Expect(collectUnits(dags[ix])).To(Equal(allUnits))
				}
			})
		})

		Context("when all dags contain different units and network is a ring", func() {

			It("should after enough long time make all dags contain same units", func() {
				nProc := 10
				connectivity := prepareRingConnectivity(nProc)
				netservs = newNetwork(nProc, connectivity)
				dags = prepareSmallDags(nProc)
				init(10 * time.Millisecond)

				// expected number of rounds to cover whole network
				expectedTime := 2 * nProc
				performTest(expectedTime * 5)

				allUnits := collectUnits(dags[0])
				for ix := range dags[1:] {
					Expect(collectUnits(dags[ix])).To(Equal(allUnits))
				}
			})
		})

	})
})
