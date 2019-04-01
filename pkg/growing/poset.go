package growing

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// An implementation of Poset that is intended to be used during poset creation.
type Poset struct {
	nProcesses int
	units      *unitBag
	primeUnits map[int]gomel.SlottedUnits
	maxUnits   gomel.SlottedUnits
	adders     []chan *unitBuilt
	newMaximal chan gomel.Unit
	tasks      sync.WaitGroup
}

// Constructs a poset for the given amount of processes.
func NewPoset(n int) *Poset {
	adders := make([]chan *unitBuilt, n, n)
	for k := range adders {
		// TODO: magic number
		adders[k] = make(chan *unitBuilt, 10)
	}
	initialPrimeUnits := map[int]gomel.SlottedUnits{}
	for i := 0; i < 10; i++ {
		initialPrimeUnits[i] = newSlottedUnits(n)
	}
	newPoset := &Poset{
		nProcesses: n,
		units:      &unitBag{},
		primeUnits: initialPrimeUnits,
		maxUnits:   newSlottedUnits(n),
		adders:     adders,
	}
	for k := range adders {
		go newPoset.adder(adders[k])
		newPoset.tasks.Add(1)
	}
	return newPoset
}

// Checks whether u1 < u2.
func (p *Poset) Below(u1 gomel.Unit, u2 gomel.Unit) bool {
	return true
}

// Returns the prime units at the requested level, indexed by their creator ids.
func (p *Poset) PrimeUnits(level int) gomel.SlottedUnits {
	return p.primeUnits[level]
}

// Returns the maximal units created by respective processes.
func (p *Poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return p.maxUnits
}

// Stops all the goroutines spawned by this poset.
func (p *Poset) Stop() {
	for _, c := range p.adders {
		close(c)
	}
	p.tasks.Wait()
}
