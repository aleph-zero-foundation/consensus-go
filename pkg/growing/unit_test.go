package growing_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/growing"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

type dagFactory struct{}

func (dagFactory) CreateDag(dc gomel.DagConfig) gomel.Dag {
	return NewDag(&dc)
}

// collectUnits runs dfs from maximal units in the given dag and returns a map
// creator => (height => slice of units by this creator on this height)
func collectUnits(dag gomel.Dag) map[int]map[int][]gomel.Unit {
	seenUnits := make(map[gomel.Hash]bool)
	result := make(map[int]map[int][]gomel.Unit)
	for pid := 0; pid < dag.NProc(); pid++ {
		result[pid] = make(map[int][]gomel.Unit)
	}

	var dfs func(u gomel.Unit)
	dfs = func(u gomel.Unit) {
		seenUnits[*u.Hash()] = true
		if _, ok := result[u.Creator()][u.Height()]; !ok {
			result[u.Creator()][u.Height()] = []gomel.Unit{}
		}
		result[u.Creator()][u.Height()] = append(result[u.Creator()][u.Height()], u)
		for _, uParent := range u.Parents() {
			if !seenUnits[*uParent.Hash()] {
				dfs(uParent)
			}
		}
	}
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			if !seenUnits[*u.Hash()] {
				dfs(u)
			}
		}
		return true
	})
	return result
}

var _ = Describe("Units", func() {
	var (
		dag        gomel.Dag
		readingErr error
		df         dagFactory
		units      map[int]map[int][]gomel.Unit
	)

	Describe("small", func() {
		JustBeforeEach(func() {
			units = collectUnits(dag)
		})
		AfterEach(func() {
			dag.(*Dag).Stop()
		})
		Describe("Checking reflexivity of Below", func() {
			BeforeEach(func() {
				dag, readingErr = tests.CreateDagFromTestFile("../testdata/one_unit.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should return true", func() {
				u := units[0][0][0]
				Expect(u.Below(u)).To(BeTrue())
			})
		})
		Describe("Checking lack of symmetry of Below", func() {
			BeforeEach(func() {
				dag, readingErr = tests.CreateDagFromTestFile("../testdata/single_unit_with_two_parents.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should be true in one direction and false in the other", func() {
				u0 := units[0][0][0]
				u1 := units[1][0][0]
				u01 := units[0][1][0]
				Expect(u0.Below(u01)).To(BeTrue())
				Expect(u1.Below(u01)).To(BeTrue())
				Expect(u01.Below(u0)).To(BeFalse())
				Expect(u01.Below(u1)).To(BeFalse())
			})
		})
		Describe("Checking transitivity of Below", func() {
			BeforeEach(func() {
				dag, readingErr = tests.CreateDagFromTestFile("../testdata/six_units.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should be true if two relations are true", func() {
				u0 := units[0][0][0]
				u01 := units[0][1][0]
				u02 := units[0][2][0]
				u21 := units[2][1][0]

				Expect(u0.Below(u01)).To(BeTrue())
				Expect(u01.Below(u02)).To(BeTrue())
				Expect(u0.Below(u02)).To(BeTrue())
				Expect(u01.Below(u21)).To(BeTrue())
				Expect(u0.Below(u21)).To(BeTrue())
			})
		})
		Describe("Checking Below works properly for forked dealing units.", func() {
			BeforeEach(func() {
				dag, readingErr = tests.CreateDagFromTestFile("../testdata/forked_dealing.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should return false for both below queries.", func() {
				u0 := units[0][0][0]
				u1 := units[0][0][1]
				Expect(u0.Below(u1)).To(BeFalse())
				Expect(u1.Below(u0)).To(BeFalse())
			})
		})
		Describe("Checking Below works properly for two forks going out of one unit.", func() {
			BeforeEach(func() {
				dag, readingErr = tests.CreateDagFromTestFile("../testdata/fork_4u.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should correctly answer all pairs of below queries.", func() {
				uBase := units[0][0][0]
				u1 := units[0][1][0]
				u2 := units[0][1][1]

				Expect(uBase.Below(u1)).To(BeTrue())
				Expect(uBase.Below(u2)).To(BeTrue())
				Expect(u1.Below(uBase)).To(BeFalse())
				Expect(u2.Below(uBase)).To(BeFalse())
				Expect(u1.Below(u2)).To(BeFalse())
				Expect(u2.Below(u1)).To(BeFalse())
			})
		})
		Describe("Checking floors", func() {
			Describe("On dealing", func() {
				BeforeEach(func() {
					dag, readingErr = tests.CreateDagFromTestFile("../testdata/only_dealing.txt", df)
					Expect(readingErr).NotTo(HaveOccurred())
				})
				It("Should return floors containing one unit each", func() {
					for pid := 0; pid < dag.NProc(); pid++ {
						floor := units[pid][0][0].Floor()
						for pid2, myFloor := range floor {
							if pid2 == pid {
								Expect(len(myFloor)).To(Equal(1))
								Expect(myFloor[0].Hash()).To(Equal(units[pid][0][0].Hash()))
							} else {
								Expect(len(myFloor)).To(Equal(0))
							}
						}
					}
				})
			})
			Describe("On a single unit with two parents", func() {
				BeforeEach(func() {
					dag, readingErr = tests.CreateDagFromTestFile("../testdata/single_unit_with_two_parents.txt", df)
					Expect(readingErr).NotTo(HaveOccurred())
				})
				It("Should contain correct floor", func() {
					floor := units[0][1][0].Floor()
					Expect(len(floor[0])).To(Equal(1))
					Expect(floor[0][0].Hash()).To(Equal(units[0][1][0].Hash()))
					Expect(len(floor[1])).To(Equal(1))
					Expect(floor[1][0].Hash()).To(Equal(units[1][0][0].Hash()))
				})
			})
			Describe("When seeing a fork", func() {
				BeforeEach(func() {
					dag, readingErr = tests.CreateDagFromTestFile("../testdata/fork_accepted.txt", df)
					Expect(readingErr).NotTo(HaveOccurred())
				})
				It("Should contain both versions", func() {
					floor := units[3][1][0].Floor()
					Expect(len(floor[0])).To(Equal(2))
					for version := 0; version < 2; version++ {
						inside := false
						for _, u := range floor[0] {
							if u.Hash() == units[0][0][version].Hash() {
								inside = true
							}
						}
						Expect(inside).To(BeTrue())
					}
				})
			})
			Describe("On a chain with 9 consecutive dealing units as the other parent ", func() {
				BeforeEach(func() {
					dag, readingErr = tests.CreateDagFromTestFile("../testdata/chain.txt", df)
					Expect(readingErr).NotTo(HaveOccurred())
				})
				It("Should contain all dealing units in floor", func() {
					floor := units[0][9][0].Floor()
					Expect(len(floor[0])).To(Equal(1))
					Expect(floor[0][0].Hash()).To(Equal(units[0][9][0].Hash()))
					for pid := 1; pid < 10; pid++ {
						Expect(len(floor[pid])).To(Equal(1))
						Expect(floor[pid][0].Hash()).To(Equal(units[pid][0][0].Hash()))
					}
				})
			})
		})
	})
})
