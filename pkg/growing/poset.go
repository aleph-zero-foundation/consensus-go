package growing

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

// Poset that is intended to be used during poset creation.
type Poset struct {
	nProcesses int
	units      *unitBag
	primeUnits *levelMap
	maxUnits   gomel.SlottedUnits
	adders     []chan *unitBuilt
	tasks      sync.WaitGroup
	pubKeys    []gomel.PublicKey
	tcByHash   *tcBag
}

// NewPoset constructs a poset using given public keys of processes.
func NewPoset(config *gomel.PosetConfig) *Poset {
	pubKeys := config.Keys
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
		tcByHash:   newTcBag(),
	}
	newPoset.tasks.Add(len(adders))
	for k := range adders {
		go newPoset.adder(adders[k])
	}
	return newPoset
}

// AddThresholdCoin adds threshold coin to the poset
func (p *Poset) AddThresholdCoin(h *gomel.Hash, tc *tcoin.ThresholdCoin) {
	p.tcByHash.add(h, tc)
}

// RemoveThresholdCoin removes threshold coin from the poset
func (p *Poset) RemoveThresholdCoin(h *gomel.Hash) {
	p.tcByHash.remove(h)
}

// ThresholdCoin returns local threshold coin dealt by dealing unit having given hash
// nil for hashes of non-dealing units
func (p *Poset) ThresholdCoin(h *gomel.Hash) *tcoin.ThresholdCoin {
	return p.tcByHash.get(h)
}

// GetCRP is a dummy implementation of a common random permutation
// TODO: implement
func (p *Poset) GetCRP(level int) []int {
	permutation := make([]int, p.NProc())
	for i := 0; i < p.NProc(); i++ {
		permutation[i] = (i + level) % p.NProc()
	}
	return permutation
}

// IsQuorum checks if subsetSize forms a quorum amongst all nProcesses.
func IsQuorum(nProcesses int, subsetSize int) bool {
	return 3*subsetSize >= 2*nProcesses
}

// IsQuorum checks if the given number of processes forms a quorum amongst all processes.
func (p *Poset) IsQuorum(number int) bool {
	return IsQuorum(p.nProcesses, number)
}

// NProc returns number of processes which uses the poset
func (p *Poset) NProc() int {
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

// Get returns a slice of units corresponding to the hashes provided.
// If a unit of a given hash is not present in the poset, the value at the same index in the result is nil.
func (p *Poset) Get(hashes []*gomel.Hash) []gomel.Unit {
	return p.units.get(hashes)
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
