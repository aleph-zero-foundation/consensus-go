package creating_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/creating"
)

type poset struct {
	primeUnits   []gomel.SlottedUnits
	maximalUnits gomel.SlottedUnits
}

func (p *poset) AddUnit(_ gomel.Preunit, _ func(gomel.Preunit, gomel.Unit, error)) {}

func (p *poset) PrimeUnits(level int) gomel.SlottedUnits {
	return p.primeUnits[level]
}

func (p *poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return p.maximalUnits
}

func (p *poset) IsQuorum(_ int) bool {
	return false
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

type unit struct {
	creator   int
	signature gomel.Signature
	hash      gomel.Hash
	height    int
	parents   []gomel.Unit
	level     int
}

func (u *unit) Below(v gomel.Unit) bool {
	toVisit := []gomel.Unit{v}
	visiting := map[gomel.Hash]bool{}
	visiting[*v.Hash()] = true
	for len(toVisit) > 0 {
		w := toVisit[0]
		toVisit = toVisit[1:]
		if w == u {
			return true
		}
		for _, p := range w.Parents() {
			if !visiting[*p.Hash()] {
				toVisit = append(toVisit, p)
				visiting[*p.Hash()] = true
			}
		}
	}
	return false
}

func (u *unit) Above(v gomel.Unit) bool {
	return v.Below(u)
}

func (u *unit) Creator() int {
	return u.creator
}

func (u *unit) Signature() gomel.Signature {
	return u.signature
}

func (u *unit) Hash() *gomel.Hash {
	return &u.hash
}

func (u *unit) Height() int {
	return u.height
}

func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

func (u *unit) Level() int {
	return u.level
}

var _ = Describe("Creating", func() {

	var (
		pu []gomel.SlottedUnits
		mu gomel.SlottedUnits
		p  *poset
	)

	BeforeEach(func() {
		mu = nil
		pu = nil
	})

	JustBeforeEach(func() {
		p = &poset{
			primeUnits:   pu,
			maximalUnits: mu,
		}
	})

	Describe("in a small poset", func() {

		var (
			nProcesses        int
			maxUnitsInPoset   []gomel.Unit
			primeUnitsInPoset []gomel.Unit
		)

		BeforeEach(func() {
			nProcesses = 4
			pu = []gomel.SlottedUnits{}
			for i := 0; i < 10; i++ {
				pu = append(pu, newSlottedUnits(nProcesses))
			}
			mu = newSlottedUnits(nProcesses)
			maxUnitsInPoset = nil
			primeUnitsInPoset = nil
		})

		JustBeforeEach(func() {
			for _, u := range maxUnitsInPoset {
				id := u.Creator()
				mu.Set(id, append(mu.Get(id), u))
			}
			for _, u := range primeUnitsInPoset {
				id := u.Creator()
				level := u.Level()
				pu[level].Set(id, append(pu[level].Get(id), u))
			}
		})

		Context("that is empty", func() {

			It("should return a dealing unit", func() {
				pu, err := NewUnit(p, 0, nProcesses)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).To(BeEmpty())
			})

			It("should return a dealing unit", func() {
				pu, err := NewUnit(p, 3, nProcesses)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(3))
				Expect(pu.Parents()).To(BeEmpty())
			})

		})

		Context("that contains a single dealing unit", func() {

			BeforeEach(func() {
				singleUnit := &unit{
					creator: 0,
					height:  0,
					parents: nil,
					level:   0,
				}
				singleUnit.hash[0] = 1
				primeUnitsInPoset = append(primeUnitsInPoset, singleUnit)
				maxUnitsInPoset = append(maxUnitsInPoset, singleUnit)
			})

			It("should return a dealing unit for a different creator", func() {
				pu, err := NewUnit(p, 3, nProcesses)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(3))
				Expect(pu.Parents()).To(BeEmpty())
			})

			It("should fail due to not enough parents for the same creator", func() {
				_, err := NewUnit(p, 0, nProcesses)
				Expect(err).To(MatchError("No legal parents for the unit."))
			})

		})

		Context("that contains two dealing units", func() {

			BeforeEach(func() {
				for id := 0; id < 2; id++ {
					someUnit := &unit{
						creator: id,
						height:  0,
						parents: nil,
						level:   0,
					}
					someUnit.hash[0] = byte(id + 1)
					primeUnitsInPoset = append(primeUnitsInPoset, someUnit)
					maxUnitsInPoset = append(maxUnitsInPoset, someUnit)
				}
			})

			It("should return a unit with these parents", func() {
				pu, err := NewUnit(p, 0, nProcesses)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).NotTo(BeEmpty())
				Expect(len(pu.Parents())).To(BeEquivalentTo(2))
				Expect(pu.Parents()[0][0]).To(BeEquivalentTo(1))
				Expect(pu.Parents()[1][0]).To(BeEquivalentTo(2))
			})

		})

		Context("that contains all the dealing units", func() {

			BeforeEach(func() {
				for id := 0; id < nProcesses; id++ {
					someUnit := &unit{
						creator: id,
						height:  0,
						parents: nil,
						level:   0,
					}
					someUnit.hash[0] = byte(id + 1)
					primeUnitsInPoset = append(primeUnitsInPoset, someUnit)
					maxUnitsInPoset = append(maxUnitsInPoset, someUnit)
				}
			})

			It("should return a unit with some parents", func() {
				pu, err := NewUnit(p, 0, nProcesses)
				Expect(err).NotTo(HaveOccurred())
				Expect(pu.Creator()).To(Equal(0))
				Expect(pu.Parents()).NotTo(BeEmpty())
				Expect(len(pu.Parents()) > 1).To(BeTrue())
				Expect(pu.Parents()[0][0]).To(BeEquivalentTo(1))
			})

		})

	})

})
