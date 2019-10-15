package check_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	. "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type preunitMock struct {
	creator     uint16
	signature   gomel.Signature
	hash        gomel.Hash
	controlHash gomel.Hash
	parents     []*gomel.Hash
	data        []byte
	rsData      []byte
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

func (pu *preunitMock) ControlHash() *gomel.Hash {
	return &pu.controlHash
}

func (pu *preunitMock) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

func (pu *preunitMock) Parents() []*gomel.Hash {
	return pu.parents
}

type defaultChecksFactory struct{}

func (defaultChecksFactory) CreateDag(dc config.Dag) gomel.Dag {
	dag, _ := check.Signatures(New(uint16(len(dc.Keys))), dc.Keys)
	return check.ForkerMuting(check.NoSelfForkingEvidence(check.ParentConsistency(check.BasicCompliance(dag))))
}

type noSelfForkingEvidenceFactory struct{}

func (noSelfForkingEvidenceFactory) CreateDag(dc config.Dag) gomel.Dag {
	return check.NoSelfForkingEvidence(New(uint16(len(dc.Keys))))
}

var _ = Describe("Dag", func() {

	var (
		nProcesses uint16
		dag        gomel.Dag
		addFirst   [][]*preunitMock
		pubKeys    []gomel.PublicKey
		privKeys   []gomel.PrivateKey
	)

	BeforeEach(func() {
		nProcesses = 0
		dag = nil
		addFirst = nil
	})

	JustBeforeEach(func() {
		for _, pus := range addFirst {
			for _, pu := range pus {
				pu.SetSignature(privKeys[pu.creator].Sign(pu))
				_, err := gomel.AddUnit(dag, pu)
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	Describe("with default checks", func() {

		BeforeEach(func() {
			nProcesses = 4
			pubKeys = make([]gomel.PublicKey, nProcesses, nProcesses)
			privKeys = make([]gomel.PrivateKey, nProcesses, nProcesses)
			for i := uint16(0); i < nProcesses; i++ {
				pubKeys[i], privKeys[i], _ = signing.GenerateKeys()
			}
			dag = defaultChecksFactory{}.CreateDag(config.Dag{Keys: pubKeys})
		})

		Describe("HasForkingEvidence works properly in case of forks even when combined floors is not an evidence of forking", func() {

			It("should confirm that a unit exploiting it is a self-forking evidence", func() {
				_, err := tests.CreateDagFromTestFile("../../testdata/dags/10/self_forking_evidence.txt", noSelfForkingEvidenceFactory{})
				Expect(err).To(Equal(gomel.NewComplianceError("A unit is evidence of self forking")))
			})
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
				parentHashes = make([]*gomel.Hash, nProcesses)
			})

			JustBeforeEach(func() {
				addedUnit.creator = addedCreator
				addedUnit.hash = addedHash
				addedUnit.parents = parentHashes
				addedUnit.SetSignature(privKeys[addedUnit.creator].Sign(addedUnit))
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
						Expect(result.Signature()).To(Equal(addedUnit.Signature()))
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
						pu.parents = make([]*gomel.Hash, nProcesses)
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should be added as a second dealing unit", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(err).NotTo(HaveOccurred())
						Expect(result.Hash()).To(Equal(addedUnit.Hash()))
						Expect(gomel.Prime(result)).To(BeTrue())
						Expect(result.Parents()[result.Creator()]).To(BeNil())
					})

				})

			})

			Context("With one parent", func() {

				BeforeEach(func() {
					addedHash[0] = 43
					parentHashes = make([]*gomel.Hash, nProcesses)
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
						pu.parents = make([]*gomel.Hash, nProcesses)
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should fail because of non prime unit", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewComplianceError("non-prime unit")))
					})

				})

			})

			Context("With three parents", func() {

				BeforeEach(func() {
					addedHash[0] = 43
					parentHashes = make([]*gomel.Hash, nProcesses)
					parentHashes[0] = &gomel.Hash{1}
					parentHashes[1] = &gomel.Hash{2}
					parentHashes[2] = &gomel.Hash{3}
				})

				Context("When the dag is empty", func() {

					It("Should fail because of lack of parents", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(3)))
					})

				})

				Context("When the dag contains one of the parents", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash = *parentHashes[0]
						pu.parents = make([]*gomel.Hash, nProcesses)
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should fail because of lack of parents", func() {
						result, err := gomel.AddUnit(dag, addedUnit)
						Expect(result).To(BeNil())
						Expect(err).To(MatchError(gomel.NewUnknownParents(2)))
					})

				})

				Context("When the dag contains all the parents", func() {

					BeforeEach(func() {
						pu1 := &preunitMock{}
						pu1.hash = *parentHashes[0]
						pu1.parents = make([]*gomel.Hash, nProcesses)
						pu2 := &preunitMock{}
						pu2.hash = *parentHashes[1]
						pu2.creator = 1
						pu2.parents = make([]*gomel.Hash, nProcesses)
						pu3 := &preunitMock{}
						pu3.hash = *parentHashes[2]
						pu3.creator = 2
						pu3.parents = make([]*gomel.Hash, nProcesses)
						addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2, pu3}}
					})

					It("Should add the unit succesfully", func() {
						_, err := gomel.AddUnit(dag, addedUnit)
						Expect(err).NotTo(HaveOccurred())
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
					pu.parents = make([]*gomel.Hash, nProcesses)
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
					pu1.parents = make([]*gomel.Hash, nProcesses)
					pu2 := &preunitMock{}
					pu2.hash[0] = 2
					pu2.creator = 1
					pu2.parents = make([]*gomel.Hash, nProcesses)
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
					pu1.parents = make([]*gomel.Hash, nProcesses)
					pu2 := &preunitMock{}
					pu2.hash[0] = 2
					pu2.creator = 0
					pu2.parents = make([]*gomel.Hash, nProcesses)
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
					pu1.parents = make([]*gomel.Hash, nProcesses)

					pu2 := &preunitMock{}
					pu2.hash[0] = 2
					pu2.creator = 1
					pu2.parents = make([]*gomel.Hash, nProcesses)

					pu3 := &preunitMock{}
					pu3.hash[0] = 3
					pu3.creator = 2
					pu3.parents = make([]*gomel.Hash, nProcesses)

					pu11 := &preunitMock{}
					pu11.hash[0] = 11
					pu11.creator = 0
					pu11.parents = make([]*gomel.Hash, nProcesses)
					pu11.parents[0] = &pu1.hash
					pu11.parents[1] = &pu2.hash
					pu11.parents[2] = &pu3.hash

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

		})

		Describe("Growing level", func() {

			Context("When the dag contains dealing units and 3 additional units", func() {

				BeforeEach(func() {
					pu0 := &preunitMock{}
					pu0.hash[0] = 1
					pu0.creator = 0
					pu0.parents = make([]*gomel.Hash, nProcesses)

					pu1 := &preunitMock{}
					pu1.hash[0] = 2
					pu1.creator = 1
					pu1.parents = make([]*gomel.Hash, nProcesses)

					pu2 := &preunitMock{}
					pu2.hash[0] = 3
					pu2.creator = 2
					pu2.parents = make([]*gomel.Hash, nProcesses)

					pu3 := &preunitMock{}
					pu3.hash[0] = 4
					pu3.creator = 3
					pu3.parents = make([]*gomel.Hash, nProcesses)

					puAbove4 := &preunitMock{}
					puAbove4.creator = 0
					puAbove4.parents = []*gomel.Hash{&pu0.hash, &pu1.hash, &pu2.hash, &pu3.hash}
					puAbove4.hash[0] = 114

					puAbove3 := &preunitMock{}
					puAbove3.creator = 1
					puAbove3.parents = make([]*gomel.Hash, nProcesses)
					puAbove3.parents[0] = &pu0.hash
					puAbove3.parents[1] = &pu1.hash
					puAbove3.parents[2] = &pu2.hash
					puAbove3.hash[0] = 113

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
				pu1, pu2, pu3 preunitMock
			)

			BeforeEach(func() {
				pu1.creator = 1
				pu1.hash[0] = 1
				pu1.parents = make([]*gomel.Hash, nProcesses)

				pu2.creator = 2
				pu2.hash[0] = 2
				pu2.parents = make([]*gomel.Hash, nProcesses)

				pu3.creator = 3
				pu3.hash[0] = 3
				pu3.parents = make([]*gomel.Hash, nProcesses)

				addFirst = [][]*preunitMock{[]*preunitMock{&pu1, &pu2, &pu3}}
			})

			Describe("check valid unit", func() {

				It("should confirm that a unit is valid", func() {
					validUnit := pu1
					validUnit.hash[0] = 4
					validUnit.parents = make([]*gomel.Hash, nProcesses)
					validUnit.parents[1] = &pu1.hash
					validUnit.parents[2] = &pu2.hash
					validUnit.parents[3] = &pu3.hash

					validUnit.SetSignature(privKeys[validUnit.creator].Sign(&validUnit))

					_, err := gomel.AddUnit(dag, &validUnit)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})
})