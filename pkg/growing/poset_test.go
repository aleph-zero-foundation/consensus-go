package growing_test

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	. "gitlab.com/alephledger/consensus-go/pkg/growing"
)

type preunitMock struct {
	creator   int
	signature gomel.Signature
	hash      gomel.Hash
	parents   []*gomel.Hash
	data      []byte
	cs        *tcoin.CoinShare
	tcData    []byte
}

func (pu *preunitMock) ThresholdCoinData() []byte {
	return pu.tcData
}

func (pu *preunitMock) CoinShare() *tcoin.CoinShare {
	return pu.cs
}

func (pu *preunitMock) Data() []byte {
	return pu.data
}

func (pu *preunitMock) Creator() int {
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

var _ = Describe("Poset", func() {

	var (
		nProcesses int
		poset      *Poset
		addFirst   [][]*preunitMock
		wg         sync.WaitGroup
		pubKeys    []gomel.PublicKey
		privKeys   []gomel.PrivateKey
	)

	AwaitAddUnit := func(pu gomel.Preunit, wg *sync.WaitGroup) {
		wg.Add(1)
		poset.AddUnit(pu, func(_ gomel.Preunit, _ gomel.Unit, err error) {
			defer GinkgoRecover()
			defer wg.Done()
			Expect(err).NotTo(HaveOccurred())
		})
	}

	BeforeEach(func() {
		nProcesses = 0
		poset = nil
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

	Describe("small", func() {

		BeforeEach(func() {
			nProcesses = 4
			pubKeys = make([]gomel.PublicKey, nProcesses, nProcesses)
			privKeys = make([]gomel.PrivateKey, nProcesses, nProcesses)
			for i := 0; i < nProcesses; i++ {
				pubKeys[i], privKeys[i], _ = signing.GenerateKeys()
			}
			poset = NewPoset(&gomel.PosetConfig{Keys: pubKeys})
		})

		AfterEach(func() {
			poset.Stop()
		})

		Describe("Adding units", func() {

			var (
				addedUnit    *preunitMock
				addedCreator int
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

				Context("When the poset is empty", func() {

					It("Should be added as a dealing unit", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).NotTo(HaveOccurred())
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Hash()).To(Equal(addedUnit.Hash()))
							Expect(gomel.Prime(result)).To(BeTrue())
							close(done)
						})
					})

				})

				Context("When the poset already contains the unit", func() {

					JustBeforeEach(func() {
						AwaitAddUnit(addedUnit, &wg)
						wg.Wait()
					})

					It("Should report that fact", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewDuplicateUnit(poset.Get([]*gomel.Hash{pu.Hash()})[0])))
							close(done)
						})
					})

				})

				Context("When the poset contains another parentless unit for this process", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash[0] = 1
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should be added as a second dealing unit", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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

				Context("When the poset is empty", func() {

					It("Should fail because of lack of parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewDataError("Missing parent")))
							close(done)
						})
					})

				})

				Context("When the poset contains the parent", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash = *parentHashes[0]
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should fail because of too few parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
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

				Context("When the poset is empty", func() {

					It("Should fail because of lack of parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewDataError("Missing parent")))
							close(done)
						})
					})

				})

				Context("When the poset contains one of the parents", func() {

					BeforeEach(func() {
						pu := &preunitMock{}
						pu.hash = *parentHashes[0]
						addFirst = [][]*preunitMock{[]*preunitMock{pu}}
					})

					It("Should fail because of lack of parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(gomel.NewDataError("Missing parent")))
							close(done)
						})
					})

				})

				Context("When the poset contains all the parents", func() {

					BeforeEach(func() {
						pu1 := &preunitMock{}
						pu1.hash = *parentHashes[0]
						pu2 := &preunitMock{}
						pu2.hash = *parentHashes[1]
						pu2.creator = 1
						addFirst = [][]*preunitMock{[]*preunitMock{pu1, pu2}}
					})

					It("Should add the unit successfully", func(done Done) {
						poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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

					Context("When the poset already contains the unit", func() {

						JustBeforeEach(func() {
							AwaitAddUnit(addedUnit, &wg)
							wg.Wait()
						})

						It("Should report that fact", func(done Done) {
							poset.AddUnit(addedUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
								defer GinkgoRecover()
								Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
								Expect(result).To(BeNil())
								Expect(err).To(MatchError(gomel.NewDuplicateUnit(poset.Get([]*gomel.Hash{pu.Hash()})[0])))
								close(done)
							})
						})

					})

				})

			})

		})

		Describe("Retrieving units", func() {

			Context("When the poset is empty", func() {

				It("Should not return any maximal units", func() {
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					for i := 0; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should not return any prime units", func() {
					for l := 0; l < 10; l++ {
						primeUnits := poset.PrimeUnits(l)
						Expect(primeUnits).NotTo(BeNil())
						for i := 0; i < nProcesses; i++ {
							Expect(len(primeUnits.Get(i))).To(BeZero())
						}
					}
				})

			})

			Context("When the poset already contains one unit", func() {

				BeforeEach(func() {
					pu := &preunitMock{}
					pu.hash[0] = 1
					pu.creator = 0
					addFirst = [][]*preunitMock{[]*preunitMock{pu}}
				})

				It("Should return it as the only maximal unit", func() {
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(1))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return it as the only prime unit", func() {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(1))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

			Context("When the poset contains two units created by different processes", func() {

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
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(1))
					Expect(len(maxUnits.Get(1))).To(Equal(1))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(maxUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return both of them as the respective prime units", func() {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(1))
					Expect(len(primeUnits.Get(1))).To(Equal(1))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

			Context("When the poset contains two units created by the same process", func() {

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
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(2))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(maxUnits.Get(0)[1].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return both of them as the respective prime units", func() {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(2))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(0)[1].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

			Context("When the poset contains a unit above another one", func() {

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
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits).NotTo(BeNil())
					Expect(len(maxUnits.Get(0))).To(Equal(1))
					Expect(len(maxUnits.Get(1))).To(Equal(1))
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[1][0].Hash()))
					Expect(maxUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return both of the parents as the respective prime units and not the top unit", func() {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits).NotTo(BeNil())
					Expect(len(primeUnits.Get(0))).To(Equal(1))
					Expect(len(primeUnits.Get(1))).To(Equal(1))
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
					primeUnits = poset.PrimeUnits(1)
					Expect(primeUnits).NotTo(BeNil())
					for i := 0; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

		})

		Describe("Growing level", func() {

			Context("When the poset contains dealing units and 3 additional units", func() {

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
					primeUnits := poset.PrimeUnits(1)
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

					poset.AddUnit(&validUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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
						poset.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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
						poset.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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

						pu3 := preunitMock{}
						pu3.creator = 1
						pu3.hash[0] = 4
						pu3.parents = []*gomel.Hash{&pu2.hash, &pForker2.hash}

						addFirst = [][]*preunitMock{
							[]*preunitMock{&pForker1, &pForker2, &pu1},
							[]*preunitMock{&pu2},
							[]*preunitMock{&pu3}}

						muted = &preunitMock{}
						muted.creator = 0
						muted.hash[0] = 5
						muted.parents = []*gomel.Hash{&pForker1.hash, &pu3.hash}
						muted.SetSignature(privKeys[muted.creator].Sign(muted))
					})

					It("should reject a unit", func(done Done) {
						poset.AddUnit(muted, func(pu gomel.Preunit, result gomel.Unit, err error) {
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
							poset.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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

							poset.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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

							poset.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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
						poset.AddUnit(&invalidUnit, func(pu gomel.Preunit, result gomel.Unit, err error) {
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
