package growing

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Dag that is intended to be used during dag creation.
type Dag struct {
	nProcesses int
	units      *unitBag
	primeUnits *levelMap
	maxUnits   gomel.SlottedUnits
	adders     []chan *unitBuilt
	tasks      sync.WaitGroup
	pubKeys    []gomel.PublicKey
}

// NewDag constructs a dag using given public keys of processes.
func NewDag(config *gomel.DagConfig) *Dag {
	pubKeys := config.Keys
	n := len(pubKeys)
	adders := make([]chan *unitBuilt, n, n)
	for k := range adders {
		adders[k] = make(chan *unitBuilt, 10)
	}
	newDag := &Dag{
		nProcesses: n,
		units:      newUnitBag(),
		primeUnits: newLevelMap(n, 10),
		maxUnits:   newSlottedUnits(n),
		adders:     adders,
		pubKeys:    pubKeys,
	}
	newDag.tasks.Add(len(adders))
	for k := range adders {
		go newDag.adder(adders[k])
	}
	return newDag
}

// IsQuorum checks if subsetSize forms a quorum amongst all nProcesses.
func IsQuorum(nProcesses int, subsetSize int) bool {
	return 3*subsetSize >= 2*nProcesses
}

// IsQuorum checks if the given number of processes forms a quorum amongst all processes.
func (dag *Dag) IsQuorum(number int) bool {
	return IsQuorum(dag.nProcesses, number)
}

// NProc returns number of processes which uses the dag
func (dag *Dag) NProc() int {
	return dag.nProcesses
}

// PrimeUnits returns the prime units at the requested level, indexed by their creator ids.
func (dag *Dag) PrimeUnits(level int) gomel.SlottedUnits {
	res, err := dag.primeUnits.getLevel(level)
	if err != nil {
		return newSlottedUnits(dag.nProcesses)
	}
	return res
}

// MaximalUnitsPerProcess returns the maximal units created by respective processes.
func (dag *Dag) MaximalUnitsPerProcess() gomel.SlottedUnits {
	return dag.maxUnits
}

// Get returns a slice of units corresponding to the hashes provided.
// If a unit of a given hash is not present in the dag, the value at the same index in the result is nil.
func (dag *Dag) Get(hashes []*gomel.Hash) []gomel.Unit {
	result, _ := dag.units.get(hashes)
	return result
}

// Stop stops all the goroutines spawned by this dag.
func (dag *Dag) Stop() {
	for _, c := range dag.adders {
		close(c)
	}
	dag.tasks.Wait()
}

func (dag *Dag) getPrimeUnitsAtLevelBelowUnit(level int, u gomel.Unit) []gomel.Unit {
	var result []gomel.Unit
	primes := dag.PrimeUnits(level)
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
