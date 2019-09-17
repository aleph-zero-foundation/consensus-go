package check_test

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	. "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
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

type defaultChecksFactory struct{}

func (defaultChecksFactory) CreateDag(dc gomel.DagConfig) gomel.Dag {
	dag, _ := check.Signatures(New(uint16(len(dc.Keys))), dc.Keys)
	return check.ExpandPrimes(check.ForkerMuting(check.NoSelfForkingEvidence(check.ParentDiversity(check.BasicCompliance(dag)))))
}

type noSelfForkingEvidenceFactory struct{}

func (noSelfForkingEvidenceFactory) CreateDag(dc gomel.DagConfig) gomel.Dag {
	return check.NoSelfForkingEvidence(New(uint16(len(dc.Keys))))
}

var _ = Describe("Dag", func() {

	var (
		nProcesses uint16
		dag        gomel.Dag
		addFirst   [][]*preunitMock
		wg         sync.WaitGroup
		pubKeys    []gomel.PublicKey
		privKeys   []gomel.PrivateKey
	)

	AwaitAddUnit := func(pu gomel.Preunit, wg *sync.WaitGroup) {
		wg.Add(1)
		dag.AddUnit(pu, func(_ gomel.Preunit, _ gomel.Unit, err error) {
			defer GinkgoRecover()
			defer wg.Done()
			Expect(err).NotTo(HaveOccurred())
		})
	}

	BeforeEach(func() {
		nProcesses = 0
		dag = nil
		addFirst = nil
		wg = sync.WaitGroup{}
	})

	JustBeforeEach(func() {
		for _, pus := range addFirst {
			for _, pu := range pus {
				pu.SetSignature(privKeys[pu.creator].Sign(pu))
				AwaitAddUnit(pu, &wg)
			}
			wg.Wait()
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
			dag = defaultChecksFactory{}.CreateDag(gomel.DagConfig{Keys: pubKeys})
		})

		Describe("HasForkingEvidence works properly in case of forks even when combined floors is not an evidence of forking", func() {

			It("should confirm that a unit exploiting it is a self-forking evidence", func() {
				_, err := tests.CreateDagFromTestFile("../../testdata/self_forking_evidence.txt", noSelfForkingEvidenceFactory{})
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
				parentHashes = []*gomel.Hash{}
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

					It("Should be added as a dealing unit", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).NotTo(HaveOccurred())
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Signature()).To(Equal(addedUnit.Signature()))
							Expect(gomel.Prime(result)).To(BeTrue())
							close(done)
						})
					})

				})

				Context("When the dag already contains the unit", func() {

					JustBeforeEach(func() {
						AwaitAddUnit(addedUnit, &wg)
						wg.Wait()
					})

					It("Should report that fact", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewDuplicateUnit(dag.Get([]*gomel.Hash{pu.Hash()})[0])))
							close(done)
						})
					})

				})

				Context("When the dag contains another parentless unit for this process", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash[0] = 1
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should be added as a second dealing unit", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).NotTo(HaveOccurred())
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Hash()).To(Equal(addedUnit.Hash()))
							Expect(gomel.Prime(result)).To(BeTrue())
							Expect(len(result.Parents())).To(BeZero())
							close(done)
						})
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

					It("Should fail because of lack of parents", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewUnknownParents(1)))
							close(done)
						})
					})

				})

				Context("When the dag contains the parent", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash = *parentHashes[0]
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should fail because of too few parents", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(err).To(MatchError(gomel.NewComplianceError("Not enough parents")))
							close(done)
						})
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

					It("Should fail because of lack of parents", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewUnknownParents(2)))
							close(done)
						})
					})

				})

				Context("When the dag contains one of the parents", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash = *parentHashes[0]
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should fail because of lack of parents", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewUnknownParents(1)))
							close(done)
						})
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

					It("Should add the unit successfully", func(done Done) {
						dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).NotTo(HaveOccurred())
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Hash()).To(Equal(addedUnit.Hash()))
							Expect(gomel.Prime(result)).To(BeFalse())
							Expect(*result.Parents()[0].Hash()).To(Equal(*addedUnit.Parents()[0]))
							Expect(*result.Parents()[1].Hash()).To(Equal(*addedUnit.Parents()[1]))
							close(done)
						})
					})

					Context("When the dag already contains the unit", func() {

						JustBeforeEach(func() {
							AwaitAddUnit(addedUnit, &wg)
							wg.Wait()
						})

						It("Should report that fact", func(done Done) {
							dag.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
								defer GinkgoRecover()
								Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
								Expect(result).To(BeNil())
								Expect(err).To(MatchError(gomel.NewDuplicateUnit(dag.Get([]*gomel.Hash{pu.Hash()})[0])))
								close(done)
							})
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

		Describe("check compliance", func() {

			var (
				pu1, pu2, pu3 preunitMock
			)

			BeforeEach(func() {
				pu1.creator = 1
				pu1.hash[0] = 1
				pu1.parents = nil

				pu2.creator = 2
				pu2.hash[0] = 2
				pu2.parents = nil

				pu3.creator = 3
				pu3.hash[0] = 3
				pu3.parents = nil

				addFirst = [][]*preunitMock{[]*preunitMock{&pu1, &pu2, &pu3}}
			})

			Describe("check valid unit", func() {

				It("should confirm that a unit is valid", func(done Done) {
					validUnit := pu1
					validUnit.hash[0] = 4
					validUnit.parents = []*gomel.Hash{&pu1.hash, &pu2.hash, &pu3.hash}
					(&validUnit).SetSignature(privKeys[validUnit.creator].Sign(&validUnit))

					dag.AddUnit(&validUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
						defer GinkgoRecover()
						Expect(err).NotTo(HaveOccurred())
						close(done)
					})
				})
			})

			Describe("invalid units", func() {
				var (
					invalidUnit preunitMock
				)

				JustBeforeEach(func() {
					(&invalidUnit).SetSignature(privKeys[invalidUnit.creator].Sign(&invalidUnit))
				})

				Describe("violated expand primes", func() {
					BeforeEach(func() {
						pu4 := preunitMock{}
						pu4.creator = pu1.creator
						pu4.hash[0] = 4
						pu4.parents = []*gomel.Hash{&pu1.hash, &pu2.hash, &pu3.hash}

						pu5 := preunitMock{}
						pu5.creator = pu2.creator
						pu5.hash[0] = 5
						pu5.parents = []*gomel.Hash{&pu2.hash, &pu1.hash, &pu3.hash}

						addFirst = append(addFirst, []*preunitMock{&pu4, &pu5})

						invalidUnit = preunitMock{}
						invalidUnit.creator = 0
						invalidUnit.hash[0] = 6
						invalidUnit.parents = []*gomel.Hash{&pu4.hash, &pu5.hash}
					})

					It("should reject a unit", func(done Done) {
						dag.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).To(MatchError(HavePrefix("ComplianceError")))
							close(done)
						})
					})
				})

				Describe("violated self forking evidence", func() {
					BeforeEach(func() {
						// forking dealing unit
						pu3.creator = pu1.creator

						// evidence of the first fork
						pu4 := preunitMock{}
						pu4.creator = pu1.creator
						pu4.hash[0] = 4
						pu4.parents = []*gomel.Hash{&pu1.hash, &pu2.hash}

						// evidence of the second fork
						pu5 := preunitMock{}
						pu5.creator = pu2.creator
						pu5.hash[0] = 5
						pu5.parents = []*gomel.Hash{&pu2.hash, &pu3.hash}

						addFirst = append(addFirst, []*preunitMock{&pu4, &pu5})

						// self forking evidence - merge of two previous forks
						invalidUnit.creator = pu1.creator
						invalidUnit.hash[0] = 6
						invalidUnit.parents = []*gomel.Hash{&pu4.hash, &pu5.hash}
					})

					It("should reject a unit", func(done Done) {
						dag.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).To(MatchError(HavePrefix("ComplianceError")))
							close(done)
						})
					})
				})

				Describe("violated forker muting rule", func() {
					var muted *preunitMock

					BeforeEach(func() {
						pForker1 := preunitMock{}
						pForker1.creator = 0
						pForker1.hash[0] = 0

						pForker2 := preunitMock{}
						pForker2.creator = 0
						pForker2.hash[0] = 1

						pu1 := preunitMock{}
						pu1.creator = 1
						pu1.hash[0] = 2

						pu2 := preunitMock{}
						pu2.creator = 1
						pu2.hash[0] = 3
						pu2.parents = []*gomel.Hash{&pu1.hash, &pForker1.hash}

						// we have to add some helper unit in order to satisfy 'expand_primes' rule
						fixingPrime := preunitMock{}
						fixingPrime.creator = 2
						fixingPrime.hash[0] = 4

						fixingUnit := preunitMock{}
						fixingUnit.creator = 2
						fixingUnit.hash[0] = 5
						fixingUnit.parents = []*gomel.Hash{&fixingPrime.hash, &pForker2.hash}

						pu3 := preunitMock{}
						pu3.creator = 1
						pu3.hash[0] = 6
						pu3.parents = []*gomel.Hash{&pu2.hash, &fixingUnit.hash}

						addFirst = [][]*preunitMock{
							[]*preunitMock{&pForker1, &pForker2, &pu1, &fixingPrime},
							[]*preunitMock{&pu2, &fixingUnit},
							[]*preunitMock{&pu3}}

						muted = &preunitMock{}
						muted.creator = 0
						muted.hash[0] = 7
						muted.parents = []*gomel.Hash{&pForker1.hash, &pu3.hash}
						muted.SetSignature(privKeys[muted.creator].Sign(muted))
					})

					It("should reject a unit", func(done Done) {
						dag.AddUnit(muted, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).To(MatchError(HavePrefix("ComplianceError")))
							close(done)
						})
					})
				})

				Describe("violated precheck", func() {

					Describe("invalid self predecessor", func() {

						BeforeEach(func() {
							invalidUnit = preunitMock{}
							invalidUnit.creator = 0
							invalidUnit.hash[0] = 4
							invalidUnit.parents = []*gomel.Hash{&pu1.hash, &pu2.hash}
						})

						It("should reject a unit", func(done Done) {
							dag.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
								defer GinkgoRecover()
								Expect(err).To(MatchError(HavePrefix("ComplianceError")))
								close(done)
							})
						})
					})

					Describe("invalid number of parents", func() {

						BeforeEach(func() {
							invalidUnit = preunitMock{}
							invalidUnit.creator = 1
							invalidUnit.hash[0] = 4
							invalidUnit.parents = []*gomel.Hash{&pu1.hash}
						})

						It("should reject a unit", func(done Done) {

							dag.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
								defer GinkgoRecover()
								Expect(err).To(MatchError(HavePrefix("ComplianceError")))
								close(done)
							})
						})
					})

					Describe("first parent is not self-predecessor", func() {

						BeforeEach(func() {
							invalidUnit = preunitMock{}
							invalidUnit.creator = 1
							invalidUnit.hash[0] = 4
							invalidUnit.parents = []*gomel.Hash{&pu2.hash, &pu1.hash}
						})

						It("should reject a unit", func(done Done) {

							dag.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
								defer GinkgoRecover()
								Expect(err).To(MatchError(HavePrefix("ComplianceError")))
								close(done)
							})
						})
					})
				})

				Describe("parents are not created by pairwise different process", func() {

					BeforeEach(func() {

						pu4 := preunitMock{}
						pu4.creator = pu2.creator
						pu4.hash[0] = 4
						pu4.parents = []*gomel.Hash{&pu2.hash, &pu1.hash}

						addFirst = append(addFirst, []*preunitMock{&pu4})

						invalidUnit = preunitMock{}
						invalidUnit.creator = pu3.creator
						invalidUnit.hash[0] = 5
						invalidUnit.parents = []*gomel.Hash{&pu3.hash, &pu2.hash, &pu4.hash}
					})

					It("should reject a unit", func(done Done) {
						dag.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).To(MatchError(HavePrefix("ComplianceError")))
							close(done)
						})
					})
				})
			})
		})
	})
})
