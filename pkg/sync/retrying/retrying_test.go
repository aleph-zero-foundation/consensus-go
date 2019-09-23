package retrying_test

import (
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

func pre(u gomel.Unit) gomel.Preunit {
	parents := u.Parents()
	hashes := make([]*gomel.Hash, len(parents))
	for i := 0; i < len(parents); i++ {
		hashes[i] = parents[i].Hash()
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
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(10)
		dags = make([]gomel.Dag, 10)
		adders = make([]*adder, 10)
		fetches = make([]sync.Server, 10)
		fallbacks = make([]sync.Fallback, 10)
	})

	JustBeforeEach(func() {
		for i := 0; i < 10; i++ {
			adders[i] = &adder{tests.NewAdder(dags[i]), nil}
			fetches[i], fallbacks[i] = fetch.NewServer(0, dags[i], adders[i], netservs[i], time.Second, zerolog.Nop(), 2, 5)
			fetches[i].Start()
		}
		retrService, retr = NewService(dags[0], adders[0], fallbacks[0], time.Millisecond, zerolog.Nop())
		fetches[0].SetFallback(retr)
		retrService.Start()
	})

	Describe("with only two participants", func() {

		Context("when requesting a dealing unit", func() {

			var (
				unit gomel.Unit
				pu   gomel.Preunit
			)

			BeforeEach(func() {
				dags[0], _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				for i := 1; i < 10; i++ {
					dags[i], _ = tests.CreateDagFromTestFile("../../testdata/one_unit.txt", tests.NewTestDagFactory())
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

			var (
				unit gomel.Unit
				pu   gomel.Preunit
			)
			BeforeEach(func() {
				dags[0], _ = tests.CreateDagFromTestFile("../../testdata/empty.txt", tests.NewTestDagFactory())
				for i := 1; i < 10; i++ {
					dags[i], _ = tests.CreateDagFromTestFile("../../testdata/random_10p_100u_2par_dead0.txt", tests.NewTestDagFactory())
				}
				maxes := dags[1].MaximalUnitsPerProcess()
				unit = maxes.Get(1)[0]
				pu = pre(unit)
			})

			It("should eventually add the unit", func() {
				retr.Resolve(pu)

				time.Sleep(time.Millisecond * 200)
				retrService.Stop()
				time.Sleep(time.Millisecond * 300)
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
			})

		})

	})

})
