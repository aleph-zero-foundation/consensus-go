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
		poset    a.Poset
		addFirst [][]*preunit
		wg       sync.WaitGroup
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
			poset = NewPoset(4)
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
	})

})
