package growing_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/growing"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

type posetFactory struct{}

func (posetFactory) CreatePoset(pc gomel.PosetConfig) gomel.Poset {
	return NewPoset(&pc)
}

// collectUnits runs dfs from maximal units in the given poset and returns a map
// (creator, height) => slice of units by this creator on this height
func collectUnits(p gomel.Poset) map[[2]int][]gomel.Unit {
	seenUnits := make(map[gomel.Hash]bool)
	result := make(map[[2]int][]gomel.Unit)

	var dfs func(u gomel.Unit)
	dfs = func(u gomel.Unit) {
		seenUnits[*u.Hash()] = true
		if _, ok := result[[2]int{u.Creator(), u.Height()}]; !ok {
			result[[2]int{u.Creator(), u.Height()}] = []gomel.Unit{}
		}
		result[[2]int{u.Creator(), u.Height()}] = append(result[[2]int{u.Creator(), u.Height()}], u)
		for _, uParent := range u.Parents() {
			if !seenUnits[*uParent.Hash()] {
				dfs(uParent)
			}
		}
	}
	p.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			dfs(u)
		}
		return true
	})
	return result
}

var _ = Describe("Units", func() {
	var (
		poset      gomel.Poset
		readingErr error
		pf         posetFactory
		units      map[[2]int][]gomel.Unit
	)

	Describe("small", func() {
		JustBeforeEach(func() {
			units = collectUnits(poset)
		})
		AfterEach(func() {
			poset.(*Poset).Stop()
		})
		Describe("Checking reflexivity of Below", func() {
			BeforeEach(func() {
				poset, readingErr = tests.CreatePosetFromTestFile("../testdata/one_unit.txt", pf)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should return true", func() {
				u := units[[2]int{0, 0}][0]
				Expect(u.Below(u)).To(BeTrue())
			})
		})
		Describe("Checking lack of symmetry of Below", func() {
			BeforeEach(func() {
				poset, readingErr = tests.CreatePosetFromTestFile("../testdata/single_unit_with_two_parents.txt", pf)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should be true in one direction and false in the other", func() {
				u0 := units[[2]int{0, 0}][0]
				u1 := units[[2]int{1, 0}][0]
				u01 := units[[2]int{0, 1}][0]
				Expect(u0.Below(u01)).To(BeTrue())
				Expect(u1.Below(u01)).To(BeTrue())
				Expect(u01.Below(u0)).To(BeFalse())
				Expect(u01.Below(u1)).To(BeFalse())
			})
		})
		Describe("Checking transitivity of Below", func() {
			BeforeEach(func() {
				poset, readingErr = tests.CreatePosetFromTestFile("../testdata/six_units.txt", pf)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should be true if two relations are true", func() {
				u0 := units[[2]int{0, 0}][0]
				u01 := units[[2]int{0, 1}][0]
				u02 := units[[2]int{0, 2}][0]
				u21 := units[[2]int{2, 1}][0]

				Expect(u0.Below(u01)).To(BeTrue())
				Expect(u01.Below(u02)).To(BeTrue())
				Expect(u0.Below(u02)).To(BeTrue())
				Expect(u01.Below(u21)).To(BeTrue())
				Expect(u0.Below(u21)).To(BeTrue())
			})
		})
		Describe("Checking Below works properly for forked dealing units.", func() {
			BeforeEach(func() {
				poset, readingErr = tests.CreatePosetFromTestFile("../testdata/forked_dealing.txt", pf)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should return false for both below queries.", func() {
				u0 := units[[2]int{0, 0}][0]
				u1 := units[[2]int{0, 0}][1]
				Expect(u0.Below(u1)).To(BeFalse())
				Expect(u1.Below(u0)).To(BeFalse())
			})
		})
		Describe("Checking Below works properly for two forks going out of one unit.", func() {
			BeforeEach(func() {
				poset, readingErr = tests.CreatePosetFromTestFile("../testdata/fork_4u.txt", pf)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should correctly answer all pairs of below queries.", func() {
				uBase := units[[2]int{0, 0}][0]
				u1 := units[[2]int{0, 1}][0]
				u2 := units[[2]int{0, 1}][1]

				Expect(uBase.Below(u1)).To(BeTrue())
				Expect(uBase.Below(u2)).To(BeTrue())
				Expect(u1.Below(uBase)).To(BeFalse())
				Expect(u2.Below(uBase)).To(BeFalse())
				Expect(u1.Below(u2)).To(BeFalse())
				Expect(u2.Below(u1)).To(BeFalse())
			})
		})
	})
})
