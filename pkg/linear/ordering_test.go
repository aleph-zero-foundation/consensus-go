package linear_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	. "gitlab.com/alephledger/consensus-go/pkg/linear"
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

func (p *poset) AddUnit(_ gomel.Preunit, _ func(gomel.Preunit, gomel.Unit, error)) {
}

func (p *poset) addUnit(u *unit) {
	if u.Height() == 0 {
		if len(p.primeUnits) == 0 {
			p.primeUnits = append(p.primeUnits, newSlottedUnits(p.nProcesses))
		}
		p.primeUnits[0].Set(u.Creator(), []gomel.Unit{u})
	} else {
		if u.Level() != u.Parents()[0].Level() {
			if len(p.primeUnits) <= u.Level() {
				p.primeUnits = append(p.primeUnits, newSlottedUnits(p.nProcesses))
			}
			p.primeUnits[u.Level()].Set(u.Creator(), []gomel.Unit{u})
		}
	}
	p.maximalUnits.Set(u.Creator(), []gomel.Unit{u})
}

func (p *poset) PrimeUnits(level int) gomel.SlottedUnits {
	return p.primeUnits[level]
}

func (p *poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return p.maximalUnits
}

func (p *poset) IsQuorum(number int) bool {
	return (3*number > 2*p.nProcesses)
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

type commonRandomPermutation struct {
	n int
}

func (crp *commonRandomPermutation) Get(level int) []int {
	permutation := make([]int, crp.n, crp.n)
	for i := 0; i < crp.n; i++ {
		permutation[i] = (i + level) % crp.n
	}
	return permutation
}

func newCommonRandomPermutation(n int) *commonRandomPermutation {
	return &commonRandomPermutation{
		n: n,
	}
}

type unit struct {
	creator int
	level   int
	hash    gomel.Hash
	parents []gomel.Unit
}

func newUnit(creator int, id int) *unit {
	var h gomel.Hash
	h[0] = byte(id)
	return &unit{
		creator: creator,
		level:   0,
		hash:    h,
		parents: []gomel.Unit{},
	}
}

func (u *unit) Creator() int {
	return u.creator
}

func (u *unit) Signature() gomel.Signature {
	return nil
}

func (u *unit) Hash() *gomel.Hash {
	return &u.hash
}

func (u *unit) Height() int {
	if len(u.Parents()) == 0 {
		return 0
	} else {
		return 1 + u.Parents()[0].Height()
	}
}

func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

func (u *unit) Level() int {
	return u.level
}

func setLevel(u *unit, p *poset) {
	if u.Height() == 0 {
		u.level = 0
		return
	}
	maxLevelBelow := -1
	for _, up := range u.Parents() {
		if up.Level() > maxLevelBelow {
			maxLevelBelow = up.Level()
		}
	}
	u.level = maxLevelBelow
	seenProcesses := make(map[int]bool)
	seenUnits := make(map[gomel.Hash]bool)
	seenUnits[*u.Hash()] = true
	queue := []gomel.Unit{u}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if w.Level() == maxLevelBelow {
			seenUnits[*w.Hash()] = true
			seenProcesses[w.Creator()] = true
			for _, wParent := range w.Parents() {
				if _, exists := seenUnits[*wParent.Hash()]; !exists {
					queue = append(queue, wParent)
					seenUnits[*wParent.Hash()] = true
				}
			}
		}
	}
	if p.IsQuorum(len(seenProcesses)) {
		u.level = maxLevelBelow + 1
	}
}

func (u *unit) Below(v gomel.Unit) bool {
	if *u.Hash() == *v.Hash() {
		return true
	}
	for _, w := range v.Parents() {
		if u.Below(w) {
			return true
		}
	}
	return false
}

func (u *unit) Above(v gomel.Unit) bool {
	return v.Below(u)
}

var _ = Describe("Ordering", func() {
	var (
		poset    *poset
		crp      CommonRandomPermutation
		ordering gomel.LinearOrdering
		units    []*unit
	)
	BeforeEach(func() {
		poset = newPoset(4)
		crp = newCommonRandomPermutation(4)
		ordering = NewOrdering(poset, crp)
		units = make([]*unit, 0)
	})
	Describe("AttemptTimingDecision", func() {
		Context("on empty poset", func() {
			It("should return 0", func() {
				Expect(ordering.AttemptTimingDecision()).To(Equal(0))
			})
		})
		Context("on a regular poset", func() {
			BeforeEach(func() {
				for i := 0; i < 4; i++ {
					units = append(units, newUnit(i%4, len(units)))
					setLevel(units[i], poset)
					poset.addUnit(units[i])
				}
			})
			Context("with only dealing units", func() {
				It("should return 0", func() {
					Expect(ordering.AttemptTimingDecision()).To(Equal(0))
				})
			})
			Context("with 3 levels", func() {
				BeforeEach(func() {
					for i := 4; i < 28; i++ {
						units = append(units, newUnit(i%4, len(units)))
						if i%4 != 3 {
							units[i].parents = []gomel.Unit{units[i-4], units[i-3]}
						} else {
							units[i].parents = []gomel.Unit{units[i-4], units[i-7]}
						}
						setLevel(units[i], poset)
						poset.addUnit(units[i])
					}
				})
				It("should return 1", func() {
					Expect(ordering.AttemptTimingDecision()).To(Equal(1))
				})
			})
			Context("with 5 levels", func() {
				BeforeEach(func() {
					for i := 4; i < 44; i++ {
						units = append(units, newUnit(i%4, len(units)))
						if i%4 != 3 {
							units[i].parents = []gomel.Unit{units[i-4], units[i-3]}
						} else {
							units[i].parents = []gomel.Unit{units[i-4], units[i-7]}
						}
						setLevel(units[i], poset)
						poset.addUnit(units[i])
					}
				})
				It("should return 3", func() {
					Expect(ordering.AttemptTimingDecision()).To(Equal(3))
				})
			})
			Context("with 7 levels", func() {
				BeforeEach(func() {
					for i := 4; i < 60; i++ {
						units = append(units, newUnit(i%4, len(units)))
						if i%4 != 3 {
							units[i].parents = []gomel.Unit{units[i-4], units[i-3]}
						} else {
							units[i].parents = []gomel.Unit{units[i-4], units[i-7]}
						}
						setLevel(units[i], poset)
						poset.addUnit(units[i])
					}
				})
				It("should return 5", func() {
					Expect(ordering.AttemptTimingDecision()).To(Equal(5))
				})
			})
		})
	})
})
