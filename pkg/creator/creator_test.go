package creator_test

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creator"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/core"
)

type privateKeyStub struct {
}

func (privateKeyStub) Sign(*gomel.Hash) gomel.Signature { return gomel.ZeroHash[:] }
func (privateKeyStub) Encode() string                   { return "" }

type testEpochProofBuilder struct {
	verify func(gomel.Preunit) bool
}

func newTestEpochProofBuilder() testEpochProofBuilder {
	return testEpochProofBuilder{verify: func(gomel.Preunit) bool { return true }}
}

func (epb testEpochProofBuilder) Verify(pu gomel.Preunit) bool {
	return epb.verify(pu)
}

func (testEpochProofBuilder) TryBuilding(gomel.Unit) core.Data {
	return nil
}

func (testEpochProofBuilder) BuildShare(lastTimingUnit gomel.Unit) core.Data {
	return nil
}

func newCreator(cnf config.Config, send func(gomel.Unit)) *creator.Creator {
	dataSource := tests.NewDataSource(10)
	rsData := func(int, []gomel.Unit, gomel.EpochID) []byte {
		return nil
	}
	epochProofBuilder := func(epoch gomel.EpochID) creator.EpochProofBuilder {
		return newTestEpochProofBuilder()
	}
	log := zerolog.Logger{}.Level(zerolog.Disabled)
	return creator.New(cnf, dataSource, send, rsData, epochProofBuilder, log)
}

var _ = Describe("creator", func() {
	Describe("having enough units on some level to build a new unit on the next level", func() {
		It("should create a unit on the next level", func() {

			nProc := uint16(4)
			cnf := config.Empty()
			cnf.NProc = nProc
			cnf.NumberOfEpochs = 2
			cnf.PrivateKey = privateKeyStub{}

			// we are expecting only two units
			unitRec := make(chan gomel.Unit, 2)
			send := func(u gomel.Unit) {
				unitRec <- u
			}

			creator := newCreator(cnf, send)

			Expect(creator).NotTo(BeNil())

			alerter := gomel.NopAlerter()

			unitBelt := make(chan gomel.Unit, 1)
			lastTiming := make(chan gomel.Unit, 1)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				creator.CreateUnits(unitBelt, lastTiming, alerter)
			}()

			dag, _ := tests.NewTestDagFactoryWithEpochID(gomel.EpochID(0)).CreateDag(cnf.NProc)

			parents := make([]gomel.Unit, nProc)
			for pid := uint16(1); pid < cnf.NProc; pid++ {
				crown := gomel.EmptyCrown(cnf.NProc)
				unitData := make([]byte, 8)
				rsData := make([]byte, 0)
				privateKey := privateKeyStub{}
				pu := tests.NewPreunit(pid, crown, unitData, rsData, privateKey)
				unit := tests.FromPreunit(pu, parents, dag)

				unitBelt <- unit
			}

			createdUnit := <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(0))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(0))

			createdUnit = <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(1))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(1))

			close(unitBelt)
			wg.Wait()
			Expect(len(unitRec)).To(Equal(0))
		})
	})

	Describe("setting the config.CanSkipLevel option to false", func() {
		It("should build units for each consecutive level", func() {

			nProc := uint16(4)
			cnf := config.Empty()

			cnf.NProc = nProc
			cnf.CanSkipLevel = false
			cnf.NumberOfEpochs = 2
			cnf.PrivateKey = privateKeyStub{}

			// we are expecting only two units
			unitRec := make(chan gomel.Unit, 2)
			send := func(u gomel.Unit) {
				unitRec <- u
			}

			creator := newCreator(cnf, send)

			Expect(creator).NotTo(BeNil())

			alerter := gomel.NopAlerter()

			unitBelt := make(chan gomel.Unit, 3*3)

			dag, _ := tests.NewTestDagFactoryWithEpochID(gomel.EpochID(0)).CreateDag(cnf.NProc)

			parents := make([]gomel.Unit, cnf.NProc)
			maxLevel := 2
			for level := 0; level <= maxLevel; level++ {
				newParents := make([]gomel.Unit, 1, cnf.NProc)
				for pid := uint16(1); pid < cnf.NProc; pid++ {
					crown := gomel.CrownFromParents(parents)
					unitData := make([]byte, 8)
					rsData := make([]byte, 0)
					privateKey := privateKeyStub{}
					pu := tests.NewPreunit(pid, crown, unitData, rsData, privateKey)
					unit := tests.FromPreunit(pu, parents, dag)
					newParents = append(newParents, unit)

					unitBelt <- unit
				}
				parents = newParents
			}

			lastTiming := make(chan gomel.Unit, 1)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				creator.CreateUnits(unitBelt, lastTiming, alerter)
			}()

			for level := 0; level < 4; level++ {
				createdUnit := <-unitRec

				Expect(createdUnit).NotTo(BeNil())
				Expect(createdUnit.Level()).To(Equal(level))
				Expect(createdUnit.Creator()).To(Equal(uint16(0)))
				Expect(createdUnit.Height()).To(Equal(level))
			}

			close(unitBelt)
			wg.Wait()

			Expect(len(unitRec)).To(Equal(0))
		})
	})

	Describe("setting the config.CanSkipLevel option to true", func() {
		It("should build a single unit on highest possible level", func() {

			nProc := uint16(4)
			cnf := config.Empty()
			cnf.NProc = nProc
			cnf.CanSkipLevel = true
			cnf.NumberOfEpochs = 2
			cnf.PrivateKey = privateKeyStub{}

			unitRec := make(chan gomel.Unit, 2)
			send := func(u gomel.Unit) {
				unitRec <- u
			}

			creator := newCreator(cnf, send)

			Expect(creator).NotTo(BeNil())

			alerter := gomel.NopAlerter()

			unitBelt := make(chan gomel.Unit, 9)

			dag, _ := tests.NewTestDagFactoryWithEpochID(gomel.EpochID(0)).CreateDag(cnf.NProc)

			parents := make([]gomel.Unit, cnf.NProc)
			maxLevel := 2
			for level := 0; level <= maxLevel; level++ {
				newParents := make([]gomel.Unit, 1, cnf.NProc)
				for pid := uint16(1); pid < cnf.NProc; pid++ {
					crown := gomel.CrownFromParents(parents)
					unitData := make([]byte, 8)
					rsData := make([]byte, 0)
					privateKey := privateKeyStub{}
					pu := tests.NewPreunit(pid, crown, unitData, rsData, privateKey)
					unit := tests.FromPreunit(pu, parents, dag)
					newParents = append(newParents, unit)

					unitBelt <- unit
				}
				parents = newParents
			}
			lastTiming := make(chan gomel.Unit, 1)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				creator.CreateUnits(unitBelt, lastTiming, alerter)
			}()

			createdUnit := <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(0))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(0))

			createdUnit = <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(maxLevel + 1))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(1))

			close(unitBelt)
			wg.Wait()
			Expect(len(unitRec)).To(Equal(0))
		})
	})

	Describe("providing a valid unit from future epoch", func() {
		It("should produce a unit of that epoch", func() {
			nProc := uint16(4)
			epoch := gomel.EpochID(7)
			cnf := config.Empty()
			cnf.NProc = nProc
			cnf.CanSkipLevel = false
			cnf.NumberOfEpochs = int(epoch) + 1
			cnf.PrivateKey = privateKeyStub{}

			unitRec := make(chan gomel.Unit, 2)
			send := func(u gomel.Unit) {
				unitRec <- u
			}

			creator := newCreator(cnf, send)

			Expect(creator).NotTo(BeNil())

			alerter := gomel.NopAlerter()

			unitBelt := make(chan gomel.Unit, 1)
			lastTiming := make(chan gomel.Unit, 1)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				creator.CreateUnits(unitBelt, lastTiming, alerter)
			}()

			dag, _ := tests.NewTestDagFactoryWithEpochID(7).CreateDag(cnf.NProc)

			parents := make([]gomel.Unit, cnf.NProc)
			crown := gomel.CrownFromParents(parents)
			unitData := make([]byte, 8)
			rsData := make([]byte, 0)
			privateKey := privateKeyStub{}
			pid := uint16(1)
			pu := tests.NewPreunitFromEpoch(epoch, pid, crown, unitData, rsData, privateKey)
			unit := tests.FromPreunit(pu, parents, dag)

			unitBelt <- unit

			createdUnit := <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(0))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(0))
			Expect(createdUnit.EpochID()).To(Equal(gomel.EpochID(0)))

			createdUnit = <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(0))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(0))
			Expect(createdUnit.EpochID()).To(Equal(epoch))

			close(unitBelt)
			wg.Wait()
			Expect(len(unitRec)).To(Equal(0))
		})
	})

	Describe("providing an invalid unit from a future epoch", func() {
		It("should not produce a unit from that epoch and keep building units from previous", func() {
			nProc := uint16(4)
			epoch := gomel.EpochID(7)
			cnf := config.Empty()
			cnf.NProc = nProc
			cnf.CanSkipLevel = false
			cnf.NumberOfEpochs = int(epoch) + 1
			cnf.PrivateKey = privateKeyStub{}

			unitRec := make(chan gomel.Unit, 2)
			send := func(u gomel.Unit) {
				unitRec <- u
			}

			dataSource := tests.NewDataSource(10)
			rsDataProvider := func(int, []gomel.Unit, gomel.EpochID) []byte {
				return nil
			}
			epochProofBuilder := func(epoch gomel.EpochID) creator.EpochProofBuilder {
				epb := newTestEpochProofBuilder()
				epb.verify = func(gomel.Preunit) bool {
					return false
				}
				return epb
			}
			log := zerolog.Logger{}.Level(zerolog.Disabled)
			creator := creator.New(cnf, dataSource, send, rsDataProvider, epochProofBuilder, log)

			Expect(creator).NotTo(BeNil())

			alerter := gomel.NopAlerter()

			unitBelt := make(chan gomel.Unit, 1)
			lastTiming := make(chan gomel.Unit, 1)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				creator.CreateUnits(unitBelt, lastTiming, alerter)
			}()

			dag, _ := tests.NewTestDagFactoryWithEpochID(7).CreateDag(cnf.NProc)

			parents := make([]gomel.Unit, cnf.NProc)
			crown := gomel.CrownFromParents(parents)
			unitData := make([]byte, 8)
			rsData := make([]byte, 0)
			privateKey := privateKeyStub{}
			pid := uint16(1)
			pu := tests.NewPreunitFromEpoch(epoch, pid, crown, unitData, rsData, privateKey)
			unit := tests.FromPreunit(pu, parents, dag)

			unitBelt <- unit

			createdUnit := <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(0))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(0))
			Expect(createdUnit.EpochID()).To(Equal(gomel.EpochID(0)))

			// build enough units for making a unit of the next level
			for pid := uint16(2); pid < cnf.NProc; pid++ {
				crown := gomel.EmptyCrown(cnf.NProc)
				unitData := make([]byte, 8)
				rsData := make([]byte, 0)
				privateKey := privateKeyStub{}
				pu := tests.NewPreunit(pid, crown, unitData, rsData, privateKey)
				unit := tests.FromPreunit(pu, parents, dag)

				unitBelt <- unit
			}

			createdUnit = <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(1))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(1))
			Expect(createdUnit.EpochID()).To(Equal(gomel.EpochID(0)))

			close(unitBelt)
			wg.Wait()
			Expect(len(unitRec)).To(Equal(0))
		})
	})

	Describe("having not enough units on some level", func() {
		It("should not create new units", func() {
			nProc := uint16(4)
			cnf := config.Empty()
			cnf.NProc = nProc
			cnf.NumberOfEpochs = 2
			cnf.PrivateKey = privateKeyStub{}

			// we are expecting only two units
			unitRec := make(chan gomel.Unit, 2)
			send := func(u gomel.Unit) {
				unitRec <- u
			}

			creator := newCreator(cnf, send)

			Expect(creator).NotTo(BeNil())

			alerter := gomel.NopAlerter()

			unitBelt := make(chan gomel.Unit)
			lastTiming := make(chan gomel.Unit)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				creator.CreateUnits(unitBelt, lastTiming, alerter)
			}()

			dag, _ := tests.NewTestDagFactoryWithEpochID(gomel.EpochID(0)).CreateDag(cnf.NProc)

			parents := make([]gomel.Unit, cnf.NProc)
			for pid := uint16(3); pid < nProc; pid++ {
				crown := gomel.EmptyCrown(cnf.NProc)
				unitData := make([]byte, 8)
				rsData := make([]byte, 0)
				privateKey := privateKeyStub{}
				pu := tests.NewPreunit(pid, crown, unitData, rsData, privateKey)
				unit := tests.FromPreunit(pu, parents, dag)

				unitBelt <- unit
			}

			createdUnit := <-unitRec

			Expect(createdUnit).NotTo(BeNil())
			Expect(createdUnit.Level()).To(Equal(0))
			Expect(createdUnit.Creator()).To(Equal(uint16(0)))
			Expect(createdUnit.Height()).To(Equal(0))

			close(unitBelt)
			wg.Wait()

			createdNew := false
			select {
			case createdUnit = <-unitRec:
				createdNew = true
			default:
			}

			Expect(createdNew).To(BeFalse())
			Expect(len(unitRec)).To(Equal(0))
		})
	})
})
