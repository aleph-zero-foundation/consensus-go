package dag_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	. "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type dagFactory struct{}

func (dagFactory) CreateDag(nProc uint16) (gomel.Dag, gomel.Adder) {
	cnf := config.Empty()
	cnf.NProc = nProc
	dag := New(cnf, gomel.EpochID(0))
	adder := tests.NewAdder(dag)
	return dag, adder
}

// collectUnits runs dfs from maximal units in the given dag and returns a map
// creator => (height => slice of units by this creator on this height)
func collectUnits(dag gomel.Dag) map[uint16]map[int][]gomel.Unit {
	seenUnits := make(map[gomel.Hash]bool)
	result := make(map[uint16]map[int][]gomel.Unit)
	for pid := uint16(0); pid < dag.NProc(); pid++ {
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
			if uParent == nil {
				continue
			}
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
		units      map[uint16]map[int][]gomel.Unit
	)

	Describe("small", func() {
		JustBeforeEach(func() {
			units = collectUnits(dag)
		})
		Describe("Checking reflexivity of Above", func() {
			BeforeEach(func() {
				dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/4/one_unit.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should return true", func() {
				u := units[0][0][0]
				Expect(gomel.Above(u, u)).To(BeTrue())
			})
		})
		Describe("Checking lack of symmetry of Above", func() {
			BeforeEach(func() {
				dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/10/single_unit_with_two_parents.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should be true in one direction and false in the other", func() {
				u0 := units[0][0][0]
				u1 := units[1][0][0]
				u01 := units[0][1][0]
				Expect(gomel.Above(u01, u0)).To(BeTrue())
				Expect(gomel.Above(u01, u1)).To(BeTrue())
				Expect(gomel.Above(u0, u01)).To(BeFalse())
				Expect(gomel.Above(u1, u01)).To(BeFalse())
			})
		})
		Describe("Checking transitivity of Above", func() {
			BeforeEach(func() {
				dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/10/six_units.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should be true if two relations are true", func() {
				u0 := units[0][0][0]
				u01 := units[0][1][0]
				u02 := units[0][2][0]
				u21 := units[2][1][0]

				Expect(gomel.Above(u01, u0)).To(BeTrue())
				Expect(gomel.Above(u02, u01)).To(BeTrue())
				Expect(gomel.Above(u02, u0)).To(BeTrue())
				Expect(gomel.Above(u21, u01)).To(BeTrue())
				Expect(gomel.Above(u21, u0)).To(BeTrue())
			})
		})
		Describe("Checking Above works properly for forked dealing units.", func() {
			BeforeEach(func() {
				dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/10/forked_dealing.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should return false for both below queries.", func() {
				u0 := units[0][0][0]
				u1 := units[0][0][1]
				Expect(gomel.Above(u0, u1)).To(BeFalse())
				Expect(gomel.Above(u1, u0)).To(BeFalse())
			})
		})
		Describe("Checking Above works properly for two forks going out of one unit.", func() {
			BeforeEach(func() {
				dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/10/fork_4u.txt", df)
				Expect(readingErr).NotTo(HaveOccurred())
			})
			It("Should correctly answer all pairs of below queries.", func() {
				uBase := units[0][0][0]
				u1 := units[0][1][0]
				u2 := units[0][1][1]

				Expect(gomel.Above(u1, uBase)).To(BeTrue())
				Expect(gomel.Above(u2, uBase)).To(BeTrue())
				Expect(gomel.Above(uBase, u1)).To(BeFalse())
				Expect(gomel.Above(uBase, u2)).To(BeFalse())
				Expect(gomel.Above(u1, u2)).To(BeFalse())
				Expect(gomel.Above(u2, u1)).To(BeFalse())
			})
		})
		Describe("Checking floors", func() {
			Describe("On dealing", func() {
				BeforeEach(func() {
					dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", df)
					Expect(readingErr).NotTo(HaveOccurred())
				})
				It("Should return floors containing no units", func() {
					for pid := uint16(0); pid < dag.NProc(); pid++ {
						for pid2 := uint16(0); pid2 < dag.NProc(); pid2++ {
							myFloor := units[pid][0][0].Floor(pid2)
							Expect(len(myFloor)).To(Equal(0))
						}
					}
				})
			})
			Describe("On a single unit with two parents", func() {
				BeforeEach(func() {
					dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/10/single_unit_with_two_parents.txt", df)
					Expect(readingErr).NotTo(HaveOccurred())
				})
				It("Should contain correct floor", func() {
					floor0 := units[0][1][0].Floor(0)
					floor1 := units[0][1][0].Floor(1)
					Expect(len(floor0)).To(Equal(1))
					Expect(floor0[0]).To(Equal(units[0][0][0]))
					Expect(len(floor1)).To(Equal(1))
					Expect(floor1[0]).To(Equal(units[1][0][0]))
				})
			})

			Describe("When seeing a fork", func() {
				It("Should return an error ambigous parents", func() {
					dag, _, readingErr = tests.CreateDagFromTestFile("../testdata/dags/10/fork_accepted.txt", df)
					Expect(readingErr).To(HaveOccurred())
					Expect(readingErr).To(MatchError("Ambiguous parents"))
				})
			})
		})
	})
})
