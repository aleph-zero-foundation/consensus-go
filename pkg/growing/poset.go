package growing

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	sign "gitlab.com/alephledger/consensus-go/pkg/crypto/signing"
)

// Poset that is intended to be used during poset creation.
type Poset struct {
	nProcesses int
	units      *unitBag
	primeUnits *levelMap
	maxUnits   gomel.SlottedUnits
	adders     []chan *unitBuilt
	tasks      sync.WaitGroup
	pubKeys    []sign.PublicKey
}

// NewPoset constructs a poset using given public keys of processes.
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

// IsQuorum checks if subsetSize forms a quorum amongst all nProcesses.
func IsQuorum(nProcesses int, subsetSize int) bool {
	return 3*subsetSize >= 2*nProcesses
}

// IsQuorum checks if the given number of processes forms a quorum amongst all processes.
func (p *Poset) IsQuorum(number int) bool {
	return IsQuorum(p.nProcesses, number)
}

// GetNProcesses returns number of processes which uses the poset
func (p *Poset) GetNProcesses() int {
	return p.nProcesses
}

// PrimeUnits returns the prime units at the requested level, indexed by their creator ids.
func (p *Poset) PrimeUnits(level int) gomel.SlottedUnits {
	res, err := p.primeUnits.getLevel(level)
	if err != nil {
		return newSlottedUnits(p.nProcesses)
	}
	return res
}

// MaximalUnitsPerProcess returns the maximal units created by respective processes.
func (p *Poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return p.maxUnits
}

// Stop stops all the goroutines spawned by this poset.
func (p *Poset) Stop() {
	for _, c := range p.adders {
		close(c)
	}
	p.tasks.Wait()
}

func (p *Poset) getPrimeUnitsAtLevelBelowUnit(level int, u gomel.Unit) []gomel.Unit {
	var result []gomel.Unit
	primes := p.PrimeUnits(level)
	primes.Iterate(func(units []gomel.Unit) bool {
		for _, prime := range units {
			if prime.Below(u) {
				result = append(result, prime)
			}
		}
		return true
	})
	return result
}
