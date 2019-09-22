package retrying_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
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
		dags     []gomel.Dag
		adders   []*adder
		fetches  []sync.QueryServer
		netservs []network.Server
		retr     sync.QueryServer
	)

	BeforeEach(func() {
		netservs = tests.NewNetwork(10)
		dags = make([]gomel.Dag, 10)
		adders = make([]*adder, 10)
		fetches = make([]sync.QueryServer, 10)
	})

	JustBeforeEach(func() {
		for i := 0; i < 10; i++ {
			adders[i] = &adder{tests.NewAdder(dags[i]), nil}
			fetches[i] = fetch.NewServer(0, dags[i], adders[i], netservs[i], time.Second, zerolog.Nop(), 2, 5)
			fetches[i].Start()
		}
		retr = NewServer(dags[0], adders[0], time.Millisecond*10, zerolog.Nop())
		retr.SetFallback(fetches[0])
		fetches[0].SetFallback(retr)
		retr.Start()
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
				retr.FindOut(pu)

				time.Sleep(time.Millisecond * 500)
				retr.StopIn()
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
				retr.FindOut(pu)

				time.Sleep(time.Millisecond * 500)
				retr.StopIn()
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
					time.Sleep(time.Millisecond * 30)
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
