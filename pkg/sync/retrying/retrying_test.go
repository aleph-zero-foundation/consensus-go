package retrying_test

import (
	snc "sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	. "gitlab.com/alephledger/consensus-go/pkg/sync/retrying"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type adder struct {
	gomel.Adder
	mx           snc.Mutex
	attemptedAdd []gomel.Preunit
}

func (a *adder) AddUnit(unit gomel.Preunit) error {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, unit)
	a.mx.Unlock()
	return a.Adder.AddUnit(unit)
}

func (a *adder) AddAntichain(units []gomel.Preunit) *gomel.AggregateError {
	a.mx.Lock()
	a.attemptedAdd = append(a.attemptedAdd, units...)
	a.mx.Unlock()
	return a.Adder.AddAntichain(units)
}

func pre(u gomel.Unit) gomel.Preunit {
	parents := u.Parents()
	hashes := make([]*gomel.Hash, len(parents))
	for i := 0; i < len(parents); i++ {
		if parents[i] != nil {
			hashes[i] = parents[i].Hash()
		} else {
			hashes[i] = nil
		}
	}
	pu := creating.NewPreunit(u.Creator(), hashes, u.Data(), u.RandomSourceData())
	pu.SetSignature(u.Signature())
	return pu
}

var _ = Describe("Protocol", func() {

	var (
		dags        []gomel.Dag
		adders      []*adder
		fetches     []sync.Server
		fallbacks   []sync.Fallback
		netservs    []network.Server
		retr        sync.Fallback
		retrService process.Service
		unit        gomel.Unit
		pu          gomel.Preunit
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(4)
		dags = make([]gomel.Dag, 4)
		adders = make([]*adder, 4)
		fetches = make([]sync.Server, 4)
		fallbacks = make([]sync.Fallback, 4)
	})

	JustBeforeEach(func() {
		for i := 0; i < 4; i++ {
			adders[i] = &adder{tests.NewAdder(dags[i]), snc.Mutex{}, nil}
			fetches[i], fallbacks[i] = fetch.NewServer(0, dags[i], adders[i], netservs[i], time.Second, zerolog.Nop(), 1, 3)
			fetches[i].Start()
		}
		retrService, retr = NewService(dags[0], adders[0], fallbacks[0], time.Millisecond, zerolog.Nop())
		fetches[0].SetFallback(retr)
		retrService.Start()
	})

	Describe("with four participants", func() {

		Context("when requesting a dealing unit", func() {

			BeforeEach(func() {
				dags[0], _ = tests.CreateDagFromTestFile("../../testdata/dags/4/empty.txt", tests.NewTestDagFactory())
				for i := 1; i < 4; i++ {
					dags[i], _ = tests.CreateDagFromTestFile("../../testdata/dags/4/one_unit.txt", tests.NewTestDagFactory())
				}
				maxes := dags[1].MaximalUnitsPerProcess()
				unit = maxes.Get(0)[0]
				pu = pre(unit)
			})

			It("should add it directly", func() {
				retr.Resolve(pu)

				time.Sleep(time.Millisecond * 300)
				retrService.Stop()
				for _, f := range fetches {
					f.StopOut()
				}
				tests.CloseNetwork(netservs)
				for _, f := range fetches {
					f.StopIn()
				}
				Expect(adders[0].attemptedAdd).To(HaveLen(1))
				Expect(adders[0].attemptedAdd[0].Creator()).To(Equal(unit.Creator()))
				Expect(adders[0].attemptedAdd[0].Signature()).To(Equal(unit.Signature()))
				Expect(adders[0].attemptedAdd[0].Data()).To(Equal(unit.Data()))
				Expect(adders[0].attemptedAdd[0].RandomSourceData()).To(Equal(unit.RandomSourceData()))
				Expect(adders[0].attemptedAdd[0].Hash()).To(Equal(unit.Hash()))
			})

		})
		Context("when requesting a unit with unknown parents", func() {

			BeforeEach(func() {
				dags[0], _ = tests.CreateDagFromTestFile("../../testdata/dags/4/one_unit.txt", tests.NewTestDagFactory())
				for i := 1; i < 4; i++ {
					dags[i], _ = tests.CreateDagFromTestFile("../../testdata/dags/4/dead0.txt", tests.NewTestDagFactory())
				}
				maxes := dags[1].MaximalUnitsPerProcess()
				unit = maxes.Get(1)[0]
				pu = pre(unit)
			})

			It("should eventually add the unit", func() {
				retr.Resolve(pu)

				time.Sleep(time.Millisecond * 2000)
				retrService.Stop()
				for _, f := range fetches {
					f.StopOut()
				}
				tests.CloseNetwork(netservs)
				for _, f := range fetches {
					f.StopIn()
				}

				uh := []*gomel.Hash{unit.Hash()}
				theUnitTransferred := dags[0].Get(uh)[0]
				for theUnitTransferred == nil {
					time.Sleep(time.Millisecond * 5)
					theUnitTransferred = dags[0].Get(uh)[0]
				}
				Expect(theUnitTransferred.Creator()).To(Equal(unit.Creator()))
				Expect(theUnitTransferred.Signature()).To(Equal(unit.Signature()))
				Expect(theUnitTransferred.Data()).To(Equal(unit.Data()))
				Expect(theUnitTransferred.RandomSourceData()).To(Equal(unit.RandomSourceData()))
				Expect(theUnitTransferred.Hash()).To(Equal(unit.Hash()))
			}, 60)

		})

	})

})
