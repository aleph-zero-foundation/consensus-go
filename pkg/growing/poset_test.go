package growing_test

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	a "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/growing"
)

type preunit struct {
	creator int
	hash    a.Hash
	parents []a.Hash
}

func (pu *preunit) Creator() int {
	return pu.creator
}

func (pu *preunit) Hash() *a.Hash {
	return &pu.hash
}

func (pu *preunit) Parents() []a.Hash {
	return pu.parents
}

var _ = Describe("Poset", func() {

	var (
		nProcesses int
		poset      a.Poset
		addFirst   [][]*preunit
		wg         sync.WaitGroup
	)

	SuccessChecker := func(_ a.Preunit, _ a.Unit, err error) {
		defer GinkgoRecover()
		defer wg.Done()
		Expect(err).NotTo(HaveOccurred())
	}

	JustBeforeEach(func() {
		wg = sync.WaitGroup{}
		for _, pus := range addFirst {
			for _, pu := range pus {
				wg.Add(1)
				poset.AddUnit(pu, SuccessChecker)
			}
			wg.Wait()
		}
	})

	Describe("small", func() {

		BeforeEach(func() {
			nProcesses = 4
			poset = NewPoset(nProcesses)
		})

		Describe("Adding units", func() {

			var (
				addedUnit    *preunit
				addedCreator int
				addedHash    a.Hash
				parentHashes []a.Hash
			)

			BeforeEach(func() {
				addedUnit = &preunit{}
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

				Context("When the poset is empty", func() {

					It("Should be added as a dealing unit", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).NotTo(HaveOccurred())
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Hash()).To(Equal(addedUnit.Hash()))
							Expect(a.Prime(result)).To(BeTrue())
							close(done)
						})
					})

				})

				Context("When the poset already contains the unit", func() {

					JustBeforeEach(func() {
						wg.Add(1)
						poset.AddUnit(addedUnit, SuccessChecker)
						wg.Wait()
					})

					It("Should report that fact", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(&a.DuplicateUnit{}))
							close(done)
						})
					})

				})

				Context("When the poset contains another parentless unit for this process", func() {

					BeforeEach(func() {
						pu := &preunit{}
						pu.hash[0] = 1
						addFirst = [][]*preunit{[]*preunit{pu}}
					})

					It("Should be added as a second dealing unit", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).NotTo(HaveOccurred())
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Hash()).To(Equal(addedUnit.Hash()))
							Expect(a.Prime(result)).To(BeTrue())
							close(done)
						})
					})

				})

			})

			Context("With one parent", func() {

				BeforeEach(func() {
					addedHash[0] = 43
					parentHashes = make([]a.Hash, 1)
					parentHashes[0][0] = 1
				})

				Context("When the poset is empty", func() {

					It("Should fail because of lack of parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(a.NewDataError("Missing parent")))
							close(done)
						})
					})

				})

				Context("When the poset contains the parent", func() {

					BeforeEach(func() {
						pu := &preunit{}
						pu.hash = parentHashes[0]
						addFirst = [][]*preunit{[]*preunit{pu}}
					})

					It("Should fail because of too few parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(a.NewDataError("Not enough parents")))
							close(done)
						})
					})

				})

			})

			Context("With two parents", func() {

				BeforeEach(func() {
					addedHash[0] = 43
					parentHashes = make([]a.Hash, 2)
					parentHashes[0][0] = 1
					parentHashes[1][0] = 2
				})

				Context("When the poset is empty", func() {

					It("Should fail because of lack of parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(a.NewDataError("Missing parent")))
							close(done)
						})
					})

				})

				Context("When the poset contains one of the parents", func() {

					BeforeEach(func() {
						pu := &preunit{}
						pu.hash = parentHashes[0]
						addFirst = [][]*preunit{[]*preunit{pu}}
					})

					It("Should fail because of lack of parents", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result).To(BeNil())
							Expect(err).To(MatchError(a.NewDataError("Missing parent")))
							close(done)
						})
					})

				})

				Context("When the poset contains all the parents", func() {

					BeforeEach(func() {
						pu1 := &preunit{}
						pu1.hash = parentHashes[0]
						pu2 := &preunit{}
						pu2.hash = parentHashes[1]
						pu2.creator = 1
						addFirst = [][]*preunit{[]*preunit{pu1, pu2}}
					})

					It("Should add the unit successfully", func(done Done) {
						poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
							defer GinkgoRecover()
							Expect(err).NotTo(HaveOccurred())
							Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
							Expect(result.Hash()).To(Equal(addedUnit.Hash()))
							Expect(a.Prime(result)).To(BeFalse())
							Expect(result.Parents()[0].Hash()).To(Equal(addedUnit.Parents()[0]))
							Expect(result.Parents()[1].Hash()).To(Equal(addedUnit.Parents()[1]))
							close(done)
						})
					})

					Context("When the poset already contains the unit", func() {

						JustBeforeEach(func() {
							wg.Add(1)
							poset.AddUnit(addedUnit, SuccessChecker)
							wg.Wait()
						})

						It("Should report that fact", func(done Done) {
							poset.AddUnit(addedUnit, func(pu a.Preunit, result a.Unit, err error) {
								defer GinkgoRecover()
								Expect(pu.Hash()).To(Equal(addedUnit.Hash()))
								Expect(result).To(BeNil())
								Expect(err).To(MatchError(&a.DuplicateUnit{}))
								close(done)
							})
						})

					})

				})

			})

		})

		Describe("Retrieving units", func() {

			Context("When the poset is empty", func() {

				It("Should not return any maximal units", func(done Done) {
					maxUnits := poset.MaximalUnitsPerProcess()
					for i := 0; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should not return any prime units", func(done Done) {
					for l := 0; l < 10; l++ {
						primeUnits := poset.PrimeUnits(l)
						for i := 0; i < nProcesses; i++ {
							Expect(len(primeUnits.Get(i))).To(BeZero())
						}
					}
				})

			})

			Context("When the poset already contains one unit", func() {

				BeforeEach(func() {
					pu := &preunit{}
					pu.hash[0] = 1
					addFirst = [][]*preunit{[]*preunit{pu}}
				})

				It("Should return it as the only maximal unit", func(done Done) {
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return it as the only prime unit", func(done Done) {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

			Context("When the poset contains two units created by different processes", func() {

				BeforeEach(func() {
					pu1 := &preunit{}
					pu1.hash[0] = 1
					pu2 := &preunit{}
					pu2.hash[1] = 2
					pu2.creator = 1
					addFirst = [][]*preunit{[]*preunit{pu1, pu2}}
				})

				It("Should return both of them as maximal units", func(done Done) {
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(maxUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return both of them as the respective prime units", func(done Done) {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

			Context("When the poset contains two units created by the same process", func() {

				BeforeEach(func() {
					pu1 := &preunit{}
					pu1.hash[0] = 1
					pu2 := &preunit{}
					pu2.hash[1] = 2
					addFirst = [][]*preunit{[]*preunit{pu1, pu2}}
				})

				It("Should return both of them as maximal units", func(done Done) {
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(maxUnits.Get(0)[1].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return both of them as the respective prime units", func(done Done) {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(0)[1].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 1; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

			Context("When the poset contains a unit above another one", func() {

				BeforeEach(func() {
					pu1 := &preunit{}
					pu1.hash[0] = 1
					pu2 := &preunit{}
					pu2.hash[1] = 2
					pu2.creator = 1
					pu11 := &preunit{}
					pu1.hash[0] = 11
					pu1.parents = []a.Hash{pu1.hash, pu2.hash}
					addFirst = [][]*preunit{[]*preunit{pu1, pu2}, []*preunit{pu11}}
				})

				It("Should return it and one of its parents as maximal units", func(done Done) {
					maxUnits := poset.MaximalUnitsPerProcess()
					Expect(maxUnits.Get(0)[0].Hash()).To(Equal(addFirst[1][0].Hash()))
					Expect(maxUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(maxUnits.Get(i))).To(BeZero())
					}
				})

				It("Should return both of the parents as the respective prime units and not the top unit", func(done Done) {
					primeUnits := poset.PrimeUnits(0)
					Expect(primeUnits.Get(0)[0].Hash()).To(Equal(addFirst[0][0].Hash()))
					Expect(primeUnits.Get(1)[0].Hash()).To(Equal(addFirst[0][1].Hash()))
					for i := 2; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
					primeUnits = poset.PrimeUnits(1)
					for i := 0; i < nProcesses; i++ {
						Expect(len(primeUnits.Get(i))).To(BeZero())
					}
				})

			})

		})
	})

})
