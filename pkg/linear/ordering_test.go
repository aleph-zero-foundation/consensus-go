package linear_test

import (
	. "gitlab.com/alephledger/consensus-go/pkg/linear"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type poset struct {
	nProcesses   int
	primeUnits   []gomel.SlottedUnits
	maximalUnits gomel.SlottedUnits
}

func newPoset(n int) *poset {
	newPoset := &poset{
		nProcesses:   n,
		primeUnits:   []gomel.SlottedUnits{},
		maximalUnits: newSlottedUnits(n),
	}
	return newPoset
}

func (p *poset) AddUnit(_ gomel.Preunit, _ func(gomel.Preunit, gomel.Unit, error)) {}

func (p *poset) PrimeUnits(level int) gomel.SlottedUnits {
	return p.primeUnits[level]
}

func (p *poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return p.maximalUnits
}

func (p *poset) IsQuorum(number int) bool {
	return (2*number > 3*p.nProcesses)
}

type slottedUnits struct {
	contents [][]gomel.Unit
}

func (su *slottedUnits) Get(id int) []gomel.Unit {
	return su.contents[id]
}

func (su *slottedUnits) Set(id int, units []gomel.Unit) {
	su.contents[id] = units
}

func (su *slottedUnits) Iterate(work func([]gomel.Unit) bool) {
	for _, units := range su.contents {
		if !work(units) {
			return
		}
	}
}

func newSlottedUnits(n int) gomel.SlottedUnits {
	return &slottedUnits{
		contents: make([][]gomel.Unit, n),
	}
}

var _ = Describe("Ordering", func() {
	var (
		poset    gomel.Poset
		crp      gomel.CommonRandomPermutation
		ordering gomel.LinearOrdering
	)
	BeforeEach(func() {
		poset = newPoset(10)
		crp = nil
		ordering = NewOrdering(poset, crp)
	})
	Describe("on empty poset", func() {
		It("Should return 0", func() {
			Expect(ordering.AttemptTimingDecision()).To(Equal(0))
		})
	})
})
