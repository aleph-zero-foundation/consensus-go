package linear_test

import (
	"github.com/rs/zerolog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/linear"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

const (
	crpFixedPrefix = 5
)

var _ = Describe("Ordering", func() {
	var (
		ordering gomel.LinearOrdering
		dag      gomel.Dag
		rs       gomel.RandomSource
		err      error
	)
	Describe("DecideTiming", func() {
		Context("On empty dag on level 0", func() {
			It("should return nil", func() {
				dag, err = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				dag = rs.Bind(dag)
				ordering = NewOrdering(dag, rs, 0, crpFixedPrefix, zerolog.Nop())
				Expect(ordering.DecideTiming()).To(BeNil())
			})
		})
		Context("On a dag with only dealing units on level 0", func() {
			It("should return nil", func() {
				dag, err = tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				dag = rs.Bind(dag)
				ordering = NewOrdering(dag, rs, 0, crpFixedPrefix, zerolog.Nop())
				Expect(ordering.DecideTiming()).To(BeNil())
			})
		})
		Context("On a very regular dag with 4 processes and 10 levels defined in regular.txt file", func() {
			BeforeEach(func() {
				dag, err = tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactoryWithChecks())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				dag = rs.Bind(dag)
				ordering = NewOrdering(dag, rs, 0, crpFixedPrefix, zerolog.Nop())
			})
			It("should decide up to 8th level", func() {
				for level := 0; level < 8; level++ {
					Expect(ordering.DecideTiming()).NotTo(BeNil())
				}
				Expect(ordering.DecideTiming()).To(BeNil())
			})
		})
	})

	Describe("TimingRound", func() {
		var timingRounds [][]gomel.Unit
		Context("On empty dag on level 0", func() {
			It("should return nil", func() {
				dag, err = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				dag = rs.Bind(dag)
				ordering = NewOrdering(dag, rs, 0, crpFixedPrefix, zerolog.Nop())
				ordering.DecideTiming()
				Expect(ordering.TimingRound(0)).To(BeNil())
			})
		})
		Context("On a very regular dag with 4 processes and 10 levels defined in regular.txt file", func() {
			BeforeEach(func() {
				dag, err = tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactoryWithChecks())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				dag = rs.Bind(dag)
				ordering = NewOrdering(dag, rs, 0, crpFixedPrefix, zerolog.Nop())
				for level := 0; level < 8; level++ {
					ordering.DecideTiming()
					thisRound := ordering.TimingRound(level)
					Expect(thisRound).NotTo(BeNil())
					timingRounds = append(timingRounds, thisRound)
				}
				Expect(ordering.TimingRound(8)).To(BeNil())
			})
			It("should on each level choose timing unit on this level", func() {
				for level := 0; level < 8; level++ {
					tu := timingRounds[level][len(timingRounds[level])-1]
					Expect(tu.Level()).To(BeNumerically("==", level))
				}
			})
			It("should sort units in order consistent with the dag order", func() {
				orderedUnits := []gomel.Unit{}
				for level := 0; level < 8; level++ {
					orderedUnits = append(orderedUnits, timingRounds[level]...)
				}
				for i := 0; i < len(orderedUnits); i++ {
					for j := i + 1; j < len(orderedUnits); j++ {
						Expect(orderedUnits[i].Above(orderedUnits[j])).To(BeFalse())
					}
				}
			})
			It("should on each level choose units that are below current timing unit but not below previous timing units", func() {
				timingUnits := []gomel.Unit{}
				for level := 0; level < 8; level++ {
					tu := timingRounds[level][len(timingRounds[level])-1]
					for _, u := range timingRounds[level] {
						for _, ptu := range timingUnits {
							Expect(ptu.Above(u)).To(BeFalse())
						}
						Expect(tu.Above(u)).To(BeTrue())
					}
					timingUnits = append(timingUnits, tu)
				}
			})
		})
	})
})
