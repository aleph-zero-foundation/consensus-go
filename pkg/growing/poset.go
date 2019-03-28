package growing

import (
	a "gitlab.com/alephledger/consensus-go/pkg"
)

type Poset struct {
	nProcesses int
	units      *unitBag
	primeUnits map[int][][]a.Unit
	maxUnits   [][]a.Unit
	adders     []chan *unitBuilt
	newMaximal chan a.Unit
}

func NewPoset(n int) *Poset {
	adders := make([]chan *unitBuilt, n, n)
	for k := range adders {
		// TODO: magic number
		adders[k] = make(chan *unitBuilt, 10)
	}
	newPoset := &Poset{
		nProcesses: n,
		units:      &unitBag{},
		primeUnits: map[int][][]a.Unit{},
		maxUnits:   make([][]a.Unit, n, n),
		adders:     adders,
		// TODO: illusion number
		newMaximal: make(chan a.Unit, n),
	}
	for k := range adders {
		go newPoset.adder(adders[k])
	}
	go newPoset.maxUpdater()
	return newPoset
}

func (p *Poset) Below(u1 a.Unit, u2 a.Unit) bool {
	return true
}

func (p *Poset) PrimeUnits(level int) [][]a.Unit {
	return p.primeUnits[level]
}

func (p *Poset) MaximalUnitsPerProcess() [][]a.Unit {
	return p.maxUnits
}
