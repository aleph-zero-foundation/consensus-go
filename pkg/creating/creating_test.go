package creating_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

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
			canSkipLevel = false
			Context("and the dag is empty", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(uint16(len(pu.Parents()))).To(Equal(dag.NProc()))
					for i := uint16(0); i < dag.NProc(); i++ {
						Expect(pu.Parents()[i]).To(BeNil())
					}
				})
			})
			Context("that contains a single dealing unit", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/one_unit.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit for a different creator", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(uint16(len(pu.Parents()))).To(Equal(dag.NProc()))
					for i := uint16(0); i < dag.NProc(); i++ {
						Expect(pu.Parents()[i]).To(BeNil())
					}
				})
				It("should fail due to not enough parents for the same creator", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains two dealing units", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/two_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return an error", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains all the dealing units", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return a unit with all the dealing units as parents", func() {
					pu, _, err := NewUnit(dag, 7, []byte{}, rs, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(pu.Creator()).To(Equal(uint16(7)))
					for i := uint16(0); i < dag.NProc(); i++ {
						u := dag.PrimeUnits(0).Get(i)[0]
						Expect(pu.Parents()[i]).To(Equal(u.Hash()))
					}
				})
			})
		})
		Context("when allowed to skip level", func() {
			canSkipLevel = true
			Context("and the dag is empty", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(uint16(len(pu.Parents()))).To(Equal(dag.NProc()))
					for i := uint16(0); i < dag.NProc(); i++ {
						Expect(pu.Parents()[i]).To(BeNil())
					}
				})
			})
			Context("that contains a single dealing unit", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/one_unit.txt", tests.NewTestDagFactory())
				})
				It("should return a dealing unit for a different creator", func() {
					pu, level, err := NewUnit(dag, 3, []byte{}, rs, canSkipLevel)
					Expect(err).NotTo(HaveOccurred())
					Expect(level).To(Equal(0))
					Expect(pu.Creator()).To(Equal(uint16(3)))
					Expect(uint16(len(pu.Parents()))).To(Equal(dag.NProc()))
					for i := uint16(0); i < dag.NProc(); i++ {
						Expect(pu.Parents()[i]).To(BeNil())
					}
				})
				It("should fail due to not enough parents for the same creator", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains two dealing units", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/two_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return an error", func() {
					_, _, err := NewUnit(dag, 0, []byte{}, rs, canSkipLevel)
					Expect(err).To(HaveOccurred())
				})
			})
			Context("that contains all the dealing units", func() {
				BeforeEach(func() {
					dag, _ = tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				})
				It("should return a unit with all the dealing units as parents", func() {
					pu, _, err := NewUnit(dag, 7, []byte{}, rs, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(pu.Creator()).To(Equal(uint16(7)))
					for i := uint16(0); i < dag.NProc(); i++ {
						u := dag.PrimeUnits(0).Get(i)[0]
						Expect(pu.Parents()[i]).To(Equal(u.Hash()))
					}
				})
			})
		})
	})
})
