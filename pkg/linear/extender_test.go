package linear_test

import (
	"github.com/rs/zerolog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	. "gitlab.com/alephledger/consensus-go/pkg/linear"
	tests "gitlab.com/alephledger/consensus-go/pkg/tests"
)

const (
	crpFixedPrefix = 5
)

var _ = Describe("Ordering", func() {
	var (
		extender *Extender
		dag      gomel.Dag
		rs       gomel.RandomSource
		err      error
	)
	Describe("NextRound", func() {
		Context("On empty dag on level 0", func() {
			It("should return nil", func() {
				dag, _, err = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				cnf := config.Empty()
				cnf.OrderStartLevel = 0
				cnf.CRPFixedPrefix = crpFixedPrefix
				output := make(chan []gomel.Unit, 1)
				extender = NewExtender(dag, rs, cnf, output, zerolog.Nop())
				Expect(extender.NextRound()).To(BeNil())
			})
		})
		Context("On a dag with only dealing units on level 0", func() {
			It("should return nil", func() {
				dag, _, err = tests.CreateDagFromTestFile("../testdata/dags/10/only_dealing.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				cnf := config.Empty()
				cnf.OrderStartLevel = 0
				cnf.CRPFixedPrefix = crpFixedPrefix
				output := make(chan []gomel.Unit, 1)
				extender = NewExtender(dag, rs, cnf, output, zerolog.Nop())
				Expect(extender.NextRound()).To(BeNil())
			})
		})
		Context("On a very regular dag with 4 processes and 10 levels defined in regular.txt file", func() {
			BeforeEach(func() {
				dag, _, err = tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactoryWithChecks())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				cnf := config.Empty()
				cnf.OrderStartLevel = 0
				cnf.CRPFixedPrefix = crpFixedPrefix
				output := make(chan []gomel.Unit, 1)
				extender = NewExtender(dag, rs, cnf, output, zerolog.Nop())
			})
			It("should decide up to 8th level", func() {
				for level := 0; level < 8; level++ {
					Expect(extender.NextRound()).NotTo(BeNil())
				}
				Expect(extender.NextRound()).To(BeNil())
			})
		})
	})

	Describe("TimingRound", func() {
		var timingRounds [][]gomel.Unit
		Context("On empty dag on level 0", func() {
			It("should return nil", func() {
				dag, _, err = tests.CreateDagFromTestFile("../testdata/dags/10/empty.txt", tests.NewTestDagFactory())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				cnf := config.Empty()
				cnf.OrderStartLevel = 0
				cnf.CRPFixedPrefix = crpFixedPrefix
				output := make(chan []gomel.Unit, 1)
				extender = NewExtender(dag, rs, cnf, output, zerolog.Nop())
				timingRound := extender.NextRound()
				Expect(timingRound).To(BeNil())
			})
		})
		Context("On a very regular dag with 4 processes and 10 levels defined in regular.txt file", func() {
			BeforeEach(func() {
				dag, _, err = tests.CreateDagFromTestFile("../testdata/dags/4/regular.txt", tests.NewTestDagFactoryWithChecks())
				Expect(err).NotTo(HaveOccurred())
				rs = tests.NewTestRandomSource()
				cnf := config.Empty()
				cnf.OrderStartLevel = 0
				cnf.CRPFixedPrefix = crpFixedPrefix
				output := make(chan []gomel.Unit, 1)
				extender = NewExtender(dag, rs, cnf, output, zerolog.Nop())
				for level := 0; level < 8; level++ {
					timingRound := extender.NextRound()
					Expect(timingRound).NotTo(BeNil())
					thisRound := timingRound.OrderedUnits()
					Expect(thisRound).NotTo(BeNil())
					timingRounds = append(timingRounds, thisRound)
				}
				timingRound := extender.NextRound()
				Expect(timingRound).To(BeNil())
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
						Expect(gomel.Above(orderedUnits[i], orderedUnits[j])).To(BeFalse())
					}
				}
			})
			It("should on each level choose units that are below current timing unit but not below previous timing units", func() {
				timingUnits := []gomel.Unit{}
				for level := 0; level < 8; level++ {
					tu := timingRounds[level][len(timingRounds[level])-1]
					for _, u := range timingRounds[level] {
						for _, ptu := range timingUnits {
							Expect(gomel.Above(ptu, u)).To(BeFalse())
						}
						Expect(gomel.Above(tu, u)).To(BeTrue())
					}
					timingUnits = append(timingUnits, tu)
				}
			})
		})
	})
})
