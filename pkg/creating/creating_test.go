package creating_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

func isDealing(pu gomel.Preunit, nProc uint16) bool {
	if len(pu.ParentsHeights()) != int(nProc) {
		return false
	}
	for _, h := range pu.ParentsHeights() {
		if p != -1 {
			return false
		}
	}
	return true
}

var _ = Describe("Creating", func() {
	Describe("in a small dag", func() {
		var (
			dag          gomel.Dag
			rs           gomel.RandomSource
			canSkipLevel bool
		)
		JustBeforeEach(func() {
			rs = tests.NewTestRandomSource()
			dag = rs.Bind(dag)
		})
		Context("when not allowed to skip level", func() {
			Context("and the dag is empty", func() {
				BeforeEach(func() {
					canSkipLevel = false
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(isDealing(pu, dag.NProc())).To(BeTrue())
				})
			})
			Context("that contains a single dealing unit", func() {
				BeforeEach(func() {
					canSkipLevel = false
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/one_unit.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit for a different creator", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(isDealing(pu, dag.NProc())).To(BeTrue())
				})
				It("should fail due to not enough parents for the same creator", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains two dealing units", func() {
				BeforeEach(func() {
					canSkipLevel = false
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/two_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return an error", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains all the dealing units", func() {
				BeforeEach(func() {
					canSkipLevel = false
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return a unit with all the dealing units as parents", func() {
					pu, _, err := NewUnit(dag, 7, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(pu.Creator()).To(Equal(uint16(7)))
					hashes := []*gomel.Hash{}
					for i := uint16(0); i < dag.NProc(); i++ {
						u := dag.PrimeUnits(0).Get(i)[0]
						Expect(pu.ParentsHeights()[i]).To(Equal(u.Height()))
						hashes = append(hashes, u.Hash())
					}
					Expect(pu.ControlHash()).To(Equal(gomel.CombineHashes(hashes)))
				})
			})
			Context("that contains two levels without one unit", func() {
				BeforeEach(func() {
					canSkipLevel = false
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/4/two_levels_without_a_unit.txt", tests.NewTestDagFactory())
				})
				It("should return a unit with all the dealing units as parents", func() {
					pu, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(pu.Creator()).To(Equal(uint16(0)))
					hashes := []*gomel.Hash{}
					for i := uint16(0); i < dag.NProc(); i++ {
						u := dag.PrimeUnits(0).Get(i)[0]
						Expect(pu.ParentsHeights()[i]).To(Equal(u.Height()))
						hashes = append(hashes, u.Hash())
					}
					Expect(pu.ControlHash()).To(Equal(gomel.CombineHashes(hashes)))
				})
			})
		})
		Context("when allowed to skip level", func() {
			Context("and the dag is empty", func() {
				BeforeEach(func() {
					canSkipLevel = true
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(isDealing(pu, dag.NProc())).To(BeTrue())
				})
			})
			Context("that contains a single dealing unit", func() {
				BeforeEach(func() {
					canSkipLevel = true
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/one_unit.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit for a different creator", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(isDealing(pu, dag.NProc())).To(BeTrue())
				})
				It("should fail due to not enough parents for the same creator", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains two dealing units", func() {
				BeforeEach(func() {
					canSkipLevel = true
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/two_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return an error", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains all the dealing units", func() {
				BeforeEach(func() {
					canSkipLevel = true
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return a unit with all the dealing units as parents", func() {
					pu, _, err := NewUnit(dag, 7, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(pu.Creator()).To(Equal(uint16(7)))
					hashes := []*gomel.Hash{}
					for i := uint16(0); i < dag.NProc(); i++ {
						u := dag.PrimeUnits(0).Get(i)[0]
						Expect(pu.ParentsHeights()[i]).To(Equal(u.Height()))
						hashes = append(hashes, u.Hash())
					}
					Expect(pu.ControlHash()).To(Equal(gomel.CombineHashes(hashes)))
				})
			})
			Context("that contains two levels without one unit", func() {
				BeforeEach(func() {
					canSkipLevel = true
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/4/two_levels_without_a_unit.txt", tests.NewTestDagFactory())
				})
				It("should return a unit that skips one level", func() {
					pu, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(pu.Creator()).To(Equal(uint16(0)))
					hashes := make([]*gomel.Hash, dag.NProc())
					for i := uint16(1); i < dag.NProc(); i++ {
						u := dag.PrimeUnits(1).Get(i)[0]
						Expect(pu.ParentsHeights()[i]).To(Equal(u.Height()))
						hashes[i] = u.Hash()
					}
					Expect(pu.ParentsHeights()[0]).To(Equal(dag.PrimeUnits(0).Get(0)[0].Height()))
					hashes[0] = dag.PrimeUnits(0).Get(0)[0].Hash()
					Expect(pu.ControlHash()).To(Equal(gomel.CombineHashes(hashes)))
				})
			})
		})
	})
})
