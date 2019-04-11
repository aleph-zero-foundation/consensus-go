package growing

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	sign "gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
)

// An implementation of Poset that is intended to be used during poset creation.
type Poset struct {
	nProcesses int
	units      *unitBag
	primeUnits *levelMap
	maxUnits   gomel.SlottedUnits
	adders     []chan *unitBuilt
	tasks      sync.WaitGroup
	pubKeys    []sign.PublicKey
}

// Constructs a poset for the given amount of processes.
func NewPoset(pubKeys []sign.PublicKey) *Poset {
	n := len(pubKeys)
	adders := make([]chan *unitBuilt, n, n)
	for k := range adders {
		// TODO: magic number
		adders[k] = make(chan *unitBuilt, 10)
	}
	newPoset := &Poset{
		nProcesses: n,
		units:      newUnitBag(),
		primeUnits: newLevelMap(n, 10),
		maxUnits:   newSlottedUnits(n),
		adders:     adders,
		pubKeys:    pubKeys,
	}
	for k := range adders {
		go newPoset.adder(adders[k])
		newPoset.tasks.Add(1)
	}
	return newPoset
}

func (p *Poset) isQuorum(number int) bool {
	return 3*number >= 2*p.nProcesses
}

// Returns the prime units at the requested level, indexed by their creator ids.
func (p *Poset) PrimeUnits(level int) gomel.SlottedUnits {
	res, err := p.primeUnits.getLevel(level)
	if err != nil {
		return newSlottedUnits(p.nProcesses)
	}
	return res
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
