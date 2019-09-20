package dag_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type preunitMock struct {
	creator   uint16
	signature gomel.Signature
	hash      gomel.Hash
	parents   []*gomel.Hash
	data      []byte
	rsData    []byte
}

func (pu *preunitMock) RandomSourceData() []byte {
	return pu.rsData
}

func (pu *preunitMock) Data() []byte {
	return pu.data
}

func (pu *preunitMock) Creator() uint16 {
	return pu.creator
}

func (pu *preunitMock) Signature() gomel.Signature {
	return pu.signature
}

func (pu *preunitMock) Hash() *gomel.Hash {
	return &pu.hash
}

func (pu *preunitMock) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

func (pu *preunitMock) Parents() []*gomel.Hash {
	return pu.parents
}

var _ = Describe("Dag", func() {

	var (
		nProcesses uint16
		dag        gomel.Dag
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
				_, err := gomel.AddUnit(dag, pu)
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	Describe("small", func() {

		BeforeEach(func() {
			nProcesses = 4
			dag = New(nProcesses)
		})

		Describe("Adding units", func() {

			var (
				addedUnit    *preunitMock
				addedCreator uint16
				addedHash    gomel.Hash
				parentHashes []*gomel.Hash
			)

			BeforeEach(func() {
				addedUnit = &preunitMock{}
				addedCreator = 0
				addedHash = gomel.Hash{}
				parentHashes = []*gomel.Hash{}
			})

			JustBeforeEach(func() {
				addedUnit.creator = addedCreator
				addedUnit.hash = addedHash
				addedUnit.parents = parentHashes
			})

			Context("With no parents", func() {

				BeforeEach(func() {
					addedHash[0] = 43
				})

				Context("When the dag is empty", func() {

					It("Should be added as a dealing unit", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(err).NotTo(HaveOccurred())
						Expect(result.Hash()).To(Equal(addedUnit.Hash()))
						Expect(gomel.Prime(result)).To(BeTrue())
					})

				})

				Context("When the dag already contains the unit", func() {

					JustBeforeEach(func() {
						_, err := gomel.AddUnit(dag, addedUnit)
						Expect(err).NotTo(HaveOccurred())
					})

					It("Should report that fact", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewDuplicateUnit(dag.Get([]*gomel.Hash{addedUnit.Hash()})[0])))
					})

				})

				Context("When the dag contains another parentless unit for this process", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash[0] = 1
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should be added as a second dealing unit", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(err).NotTo(HaveOccurred())
						Expect(result.Hash()).To(Equal(addedUnit.Hash()))
						Expect(gomel.Prime(result)).To(BeTrue())
						Expect(len(result.Parents())).To(BeZero())
					})

				})

			})

			Context("With one parent", func() {

				BeforeEach(func() {
					addedHash[0] = 43
					parentHashes = make([]*gomel.Hash, 1)
					parentHashes[0] = &gomel.Hash{1}
				})

				Context("When the dag is empty", func() {

					It("Should fail because of lack of parents", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(1)))
					})

				})

				Context("When the dag contains the parent", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash = *parentHashes[0]
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should add the unit successfully", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(err).NotTo(HaveOccurred())
						Expect(result.Hash()).To(Equal(addedUnit.Hash()))
						Expect(gomel.Prime(result)).To(BeFalse())
						Expect(*result.Parents()[0].Hash()).To(Equal(*addedUnit.Parents()[0]))
					})

				})

			})

			Context("With two parents", func() {

				BeforeEach(func() {
					addedHash[0] = 43
					parentHashes = make([]*gomel.Hash, 2)
					parentHashes[0] = &gomel.Hash{1}
					parentHashes[1] = &gomel.Hash{2}
				})

				Context("When the dag is empty", func() {

					It("Should fail because of lack of parents", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(2)))
					})

				})

				Context("When the dag contains one of the parents", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash = *parentHashes[0]
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should fail because of lack of parents", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(1)))
					})

				})

				Context("When the dag contains all the parents", func() {

					BeforeEach(func() {
						pu1 := &preunitMock{}
						pu1.hash = *parentHashes[0]
						pu2 := &preunitMock{}
						pu2.hash = *parentHashes[1]
						pu2.creator = 1
						addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2}}
					})

					It("Should add the unit successfully", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(err).NotTo(HaveOccurred())
						Expect(result.Hash()).To(Equal(addedUnit.Hash()))
						Expect(gomel.Prime(result)).To(BeFalse())
						Expect(*result.Parents()[0].Hash()).To(Equal(*addedUnit.Parents()[0]))
						Expect(*result.Parents()[1].Hash()).To(Equal(*addedUnit.Parents()[1]))
					})

					Context("When the dag already contains the unit", func() {

						JustBeforeEach(func() {
							_, err := gomel.AddUnit(dag, addedUnit)
							Expect(err).NotTo(HaveOccurred())
						})

						It("Should report that fact", func() {
							result, err := gomel.AddUnit(dag, addedUnit)
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewDuplicateUnit(dag.Get([]*gomel.Hash{addedUnit.Hash()})[0])))
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
					pu := &preunitMock{}
					pu.hash[0] = 1
					pu.creator = 0
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
					pu1 := &preunitMock{}
					pu1.hash[0] = 1
					pu1.creator = 0
					pu2 := &preunitMock{}
					pu2.hash[0] = 2
					pu2.creator = 1
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
					pu1 := &preunitMock{}
					pu1.hash[0] = 1
					pu1.creator = 0
					pu2 := &preunitMock{}
					pu2.hash[0] = 2
					pu2.creator = 0
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
					pu1 := &preunitMock{}
					pu1.hash[0] = 1
					pu1.creator = 0
					pu2 := &preunitMock{}
					pu2.hash[0] = 2
					pu2.creator = 1
					pu11 := &preunitMock{}
					pu11.hash[0] = 11
					pu11.creator = 0
					pu11.parents = []*gomel.Hash{&pu1.hash, &pu2.hash}
					addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2}, []*preunitMock{pu11}}
				})

				It("Should return it and one of its parents as maximal units", func() {
					maxUnits := dag.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(1))
					Expect(len(maxUnits.Get(1))).To(Equal(1))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[1][0].Hash()))
					Expect(maxUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := uint16(2); i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return both of the parents as the respective prime units and not the top unit", func() {
					primeUnits := dag.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(1))
					Expect(len(primeUnits.Get(1))).To(Equal(1))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := uint16(2); i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
					primeUnits = dag.PrimeUnits(1)
					Expect(primeUnits).NotTo(BeNil())
					for i := uint16(0); i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

		})

		Describe("Growing level", func() {

			Context("When the dag contains dealing units and 3 additional units", func() {

				BeforeEach(func() {
					pu0 := &preunitMock{}
					pu0.hash[0] = 1
					pu0.creator = 0
					pu1 := &preunitMock{}
					pu1.hash[0] = 2
					pu1.creator = 1
					pu2 := &preunitMock{}
					pu2.hash[0] = 3
					pu2.creator = 2
					pu3 := &preunitMock{}
					pu3.hash[0] = 4
					pu3.creator = 3

					puAbove4 := &preunitMock{}
					puAbove4.creator = 0
					puAbove4.parents = []*gomel.Hash{&pu0.hash, &pu1.hash, &pu2.hash, &pu3.hash}
					puAbove4.hash[0] = 114

					puAbove3 := &preunitMock{}
					puAbove3.creator = 1
					puAbove3.parents = []*gomel.Hash{&pu1.hash, &pu0.hash, &pu2.hash}
					puAbove3.hash[0] = 113

					puAbove2 := &preunitMock{}
					puAbove2.creator = 2
					puAbove2.parents = []*gomel.Hash{&pu2.hash, &pu0.hash}
					puAbove2.hash[0] = 112

					addFirst = [][]*preunitMock{[]*preunitMock{pu0, pu1, pu2, pu3}, []*preunitMock{puAbove4, puAbove3, puAbove2}}
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
	})
})
