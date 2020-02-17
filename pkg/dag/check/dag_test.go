package check_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
	"gitlab.com/alephledger/core-go/pkg/core"
)

type preunitMock struct {
	creator   uint16
	epochID   gomel.EpochID
	signature gomel.Signature
	hash      gomel.Hash
	crown     gomel.Crown
	data      core.Data
	rsData    []byte
}

func (pu *preunitMock) EpochID() gomel.EpochID {
	return pu.epochID
}

func (pu *preunitMock) RandomSourceData() []byte {
	return pu.rsData
}

func (pu *preunitMock) Data() core.Data {
	return pu.data
}

func (pu *preunitMock) Creator() uint16 {
	return pu.creator
}

func (pu *preunitMock) Height() int {
	return pu.crown.Heights[pu.creator] + 1
}

func (pu *preunitMock) Signature() gomel.Signature {
	return pu.signature
}

func (pu *preunitMock) Hash() *gomel.Hash {
	return &pu.hash
}

func (pu *preunitMock) View() *gomel.Crown {
	return &pu.crown
}

func (pu *preunitMock) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

func newPreunitMock(creator uint16, parentsHeights []int, parentsHashes []*gomel.Hash) *preunitMock {
	return &preunitMock{
		creator: creator,
		crown:   *gomel.NewCrown(parentsHeights, gomel.CombineHashes(parentsHashes)),
	}
}

func (pu *preunitMock) SetHash(value int) {
	pu.hash[0] = byte(value)
}

type defaultChecksFactory struct{}

func (defaultChecksFactory) CreateDag(nProc uint16) (gomel.Dag, gomel.Adder) {
	dag := New(nProc)
	check.BasicCompliance(dag)
	check.ParentConsistency(dag)
	check.NoSelfForkingEvidence(dag)
	check.ForkerMuting(dag)
	return dag, tests.NewAdder(dag)
}

type noSelfForkingEvidenceFactory struct{}

func (noSelfForkingEvidenceFactory) CreateDag(nProc uint16) (gomel.Dag, gomel.Adder) {
	dag := New(nProc)
	check.NoSelfForkingEvidence(dag)
	return dag, tests.NewAdder(dag)
}

var _ = Describe("Dag", func() {

	var (
		nProcesses uint16
		dag        gomel.Dag
		adder      gomel.Adder
		addFirst   [][]*preunitMock
	)

	BeforeEach(func() {
		nProcesses = 0
		dag = nil
		addFirst = nil

	})

	JustBeforeEach(func() {
		for _, pus := range addFirst {
			for _, pu := range pus {
				err := adder.AddUnit(pu, pu.Creator())
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	Describe("with default checks", func() {
		BeforeEach(func() {
			nProcesses = 4
			dag, adder = defaultChecksFactory{}.CreateDag(nProcesses)
		})

		Describe("HasForkingEvidence works properly in case of forks even when combined floors is not an evidence of forking", func() {
			It("should confirm that a unit exploiting it is a self-forking evidence", func() {
				_, _, err := tests.CreateDagFromTestFile("../../testdata/dags/10/self_forking_evidence.txt", noSelfForkingEvidenceFactory{})
				Expect(err).To(Equal(gomel.NewComplianceError("A unit is evidence of self forking")))
			})
		})
		Describe("Adding units", func() {
			var addedUnit *preunitMock
			Context("With no parents", func() {
				BeforeEach(func() {
					addedUnit = newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					addedUnit.SetHash(43)
				})
				Context("When the dag is empty", func() {
					It("Should be added as a dealing unit", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						result := dag.GetUnit(addedUnit.Hash())
						Expect(err).NotTo(HaveOccurred())
						Expect(result.Hash()).To(Equal(addedUnit.Hash()))
						Expect(result.Signature()).To(Equal(addedUnit.Signature()))
					})
				})
				Context("When the dag already contains the unit", func() {
					JustBeforeEach(func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						Expect(err).NotTo(HaveOccurred())
					})
					It("Should report that fact", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						Expect(err).To(MatchError(gomel.NewDuplicateUnit(dag.GetUnit(addedUnit.Hash()))))
					})
				})
				Context("When the dag contains another parentless unit for this process", func() {
					BeforeEach(func() {
						pu := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu.SetHash(1)
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})
					It("Should be added as a second dealing unit", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						result := dag.GetUnit(addedUnit.Hash())
						Expect(err).NotTo(HaveOccurred())
						Expect(result.Hash()).To(Equal(addedUnit.Hash()))
						Expect(result.Parents()[result.Creator()]).To(BeNil())
					})
				})
			})
			Context("With one parent", func() {
				BeforeEach(func() {
					parentsHashes := make([]*gomel.Hash, nProcesses)
					parentsHashes[0] = &gomel.Hash{1}
					parentsHeights := gomel.DealingHeights(nProcesses)
					parentsHeights[0] = 0

					addedUnit = newPreunitMock(0, parentsHeights, parentsHashes)
					addedUnit.SetHash(43)
				})
				Context("When the dag is empty", func() {
					It("Should fail because of lack of parents", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						result := dag.GetUnit(addedUnit.Hash())
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(1)))
					})
				})
				Context("When the dag contains the parent", func() {
					BeforeEach(func() {
						pu := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu.SetHash(1)
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})
					It("Should fail because of non prime unit", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						result := dag.GetUnit(addedUnit.Hash())
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewComplianceError("non-prime unit")))
					})
				})
			})
			Context("With three parents", func() {
				BeforeEach(func() {
					parentsHashes := make([]*gomel.Hash, nProcesses)
					parentsHashes[0] = &gomel.Hash{1}
					parentsHashes[1] = &gomel.Hash{2}
					parentsHashes[2] = &gomel.Hash{3}
					parentsHeights := []int{0, 0, 0, -1}

					addedUnit = newPreunitMock(0, parentsHeights, parentsHashes)
					addedUnit.SetHash(43)
				})
				Context("When the dag is empty", func() {
					It("Should fail because of lack of parents", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						result := dag.GetUnit(addedUnit.Hash())
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(3)))
					})
				})
				Context("When the dag contains one of the parents", func() {
					BeforeEach(func() {
						pu := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu.SetHash(1)
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})
					It("Should fail because of lack of parents", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						result := dag.GetUnit(addedUnit.Hash())
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(2)))
					})
				})
				Context("When the dag contains all the parents", func() {
					BeforeEach(func() {
						pu1 := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu1.SetHash(1)
						pu2 := newPreunitMock(1, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu2.SetHash(2)
						pu3 := newPreunitMock(2, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu3.SetHash(3)
						addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2, pu3}}
					})
					It("Should add the unit succesfully", func() {
						err := adder.AddUnit(addedUnit, addedUnit.Creator())
						Expect(err).NotTo(HaveOccurred())
					})
					Context("When the dag already contains the unit", func() {
						JustBeforeEach(func() {
							err := adder.AddUnit(addedUnit, addedUnit.Creator())
							Expect(err).NotTo(HaveOccurred())
						})
						It("Should report that fact", func() {
							err := adder.AddUnit(addedUnit, addedUnit.Creator())
							Expect(err).To(MatchError(gomel.NewDuplicateUnit(dag.GetUnit(addedUnit.Hash()))))
						})
					})
				})
			})
		})
		Describe("Retrieving units", func() {
			Context("When the dag is empty", func() {
				It("Should not return any maximal units", func() {
					maxUnits := dag.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					for i := uint16(0); i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})
				It("Should not return any prime units", func() {
					for l := 0; l < 10; l++ {
						primeUnits := dag.PrimeUnits(l)
						Expect(primeUnits).NotTo(BeNil())
						for i := uint16(0); i < nProcesses; i++ {
							Expect(len(primeUnits.Get(i))).To(BeZero())
						}
					}
				})
			})
			Context("When the dag already contains one unit", func() {
				BeforeEach(func() {
					pu := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu.SetHash(1)
					addFirst = [][]*preunitMock{[]*preunitMock{pu}}
				})
				It("Should return it as the only maximal unit", func() {
					maxUnits := dag.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(1))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					for i := uint16(1); i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})
				It("Should return it as the only prime unit", func() {
					primeUnits := dag.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(1))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					for i := uint16(1); i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})
			})
			Context("When the dag contains two units created by different processes", func() {
				BeforeEach(func() {
					pu1 := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu1.SetHash(1)
					pu2 := newPreunitMock(1, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu2.SetHash(2)
					addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2}}
				})
				It("Should return both of them as maximal units", func() {
					maxUnits := dag.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(1))
					Expect(len(maxUnits.Get(1))).To(Equal(1))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(maxUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := uint16(2); i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})
				It("Should return both of them as the respective prime units", func() {
					primeUnits := dag.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(1))
					Expect(len(primeUnits.Get(1))).To(Equal(1))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := uint16(2); i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})
			})
			Context("When the dag contains two units created by the same process", func() {
				BeforeEach(func() {
					pu1 := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu1.SetHash(1)
					pu2 := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu2.SetHash(2)
					addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2}}
				})
				It("Should return both of them as maximal units", func() {
					maxUnits := dag.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(2))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(maxUnits.Get(0)[1].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := uint16(1); i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})
				It("Should return both of them as the respective prime units", func() {
					primeUnits := dag.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(2))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(0)[1].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := uint16(1); i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})
			})
			Context("When the dag contains a unit above another one", func() {
				BeforeEach(func() {
					pu1 := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu1.SetHash(1)
					pu2 := newPreunitMock(1, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu2.SetHash(2)
					pu3 := newPreunitMock(2, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu3.SetHash(3)

					pu11 := newPreunitMock(0, []int{0, 0, 0, -1}, []*gomel.Hash{&gomel.Hash{1}, &gomel.Hash{2}, &gomel.Hash{3}, &gomel.Hash{}})
					pu11.SetHash(43)
					addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2, pu3}, []*preunitMock{pu11}}
				})

				It("Should return it and one of its parents as maximal units", func() {
					maxUnits := dag.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(1))
					Expect(len(maxUnits.Get(1))).To(Equal(1))
					Expect(len(maxUnits.Get(2))).To(Equal(1))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[1][0].Hash()))
					Expect(maxUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					Expect(maxUnits.Get(2)[0].Hash()).To(Equal(addFirst[0][2].Hash()))
					for i := uint16(3); i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return the parents as the respective prime units on level 0 and top unit as a prime unit on level 1", func() {
					primeUnits := dag.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(1))
					Expect(len(primeUnits.Get(1))).To(Equal(1))
					Expect(len(primeUnits.Get(2))).To(Equal(1))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					Expect(primeUnits.Get(2)[0].Hash()).To(Equal(addFirst[0][2].Hash()))
					for i := uint16(3); i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
					primeUnits = dag.PrimeUnits(1)
					Expect(primeUnits).NotTo(BeNil())
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[1][0].Hash()))
					for i := uint16(1); i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})
			})
			Describe("Growing level", func() {
				Context("When the dag contains dealing units and 3 additional units", func() {
					BeforeEach(func() {
						pu0 := newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu0.SetHash(1)

						pu1 := newPreunitMock(1, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu1.SetHash(2)

						pu2 := newPreunitMock(2, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu2.SetHash(3)

						pu3 := newPreunitMock(3, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
						pu3.SetHash(4)

						puAbove4 := newPreunitMock(0, []int{0, 0, 0, 0}, []*gomel.Hash{&pu0.hash, &pu1.hash, &pu2.hash, &pu3.hash})
						puAbove4.SetHash(114)

						puAbove3 := newPreunitMock(1, []int{0, 0, 0, -1}, []*gomel.Hash{&pu0.hash, &pu1.hash, &pu2.hash, &gomel.Hash{}})
						puAbove3.SetHash(113)

						addFirst = [][]*preunitMock{[]*preunitMock{pu0, pu1, pu2, pu3}, []*preunitMock{puAbove4, puAbove3}}
					})

					It("Should return exactly two prime units at level 1 (processes 0, 1).", func() {
						primeUnits := dag.PrimeUnits(1)
						Expect(primeUnits).NotTo(BeNil())

						Expect(len(primeUnits.Get(0))).To(Equal(1))
						Expect(primeUnits.Get(0)[0].Level()).To(Equal(1))

						Expect(len(primeUnits.Get(1))).To(Equal(1))
						Expect(primeUnits.Get(1)[0].Level()).To(Equal(1))

						Expect(len(primeUnits.Get(2))).To(Equal(0))
						Expect(len(primeUnits.Get(3))).To(Equal(0))
					})
				})
			})
			Describe("check compliance", func() {
				var (
					pu1, pu2, pu3 *preunitMock
				)
				BeforeEach(func() {
					pu1 = newPreunitMock(0, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu1.SetHash(1)

					pu2 = newPreunitMock(1, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu2.SetHash(2)

					pu3 = newPreunitMock(2, gomel.DealingHeights(nProcesses), make([]*gomel.Hash, nProcesses))
					pu3.SetHash(3)
					addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2, pu3}}
				})
				Describe("check valid unit", func() {
					It("should confirm that a unit is valid", func() {
						validUnit := newPreunitMock(0, []int{0, 0, 0, -1}, []*gomel.Hash{&pu1.hash, &pu2.hash, &pu3.hash, &gomel.Hash{}})
						validUnit.SetHash(4)
						err := adder.AddUnit(validUnit, validUnit.Creator())
						Expect(err).NotTo(HaveOccurred())
					})
				})
				Describe("adding a unit with different EpochID", func() {
					It("should return an error", func() {
						pu := newPreunitMock(0, []int{0, 0, 0, -1}, []*gomel.Hash{&pu1.hash, &pu2.hash, &pu3.hash, &gomel.Hash{}})
						pu.SetHash(4)
						pu.epochID = 101
						err := adder.AddUnit(pu, pu.Creator())
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})
})
