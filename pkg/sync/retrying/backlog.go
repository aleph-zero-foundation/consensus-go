package retrying

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type backlog struct {
	sync.Mutex
	backlog map[gomel.Hash]gomel.Preunit
}

func newBacklog() *backlog {
	return &backlog{
		backlog: make(map[gomel.Hash]gomel.Preunit),
	}
}

func (b *backlog) add(pu gomel.Preunit) bool {
	b.Lock()
	defer b.Unlock()
	if _, ok := b.backlog[*pu.Hash()]; ok {
		return false
	}
	b.backlog[*pu.Hash()] = pu
	return true
}

func (b *backlog) del(h *gomel.Hash) {
	b.Lock()
	defer b.Unlock()
	delete(b.backlog, *h)
}

func (b *backlog) get(h *gomel.Hash) gomel.Preunit {
	b.Lock()
	defer b.Unlock()
	return b.backlog[*h]
}

func (b *backlog) refallback(findOut func(pu gomel.Preunit)) {
	b.Lock()
	defer b.Unlock()
	for _, pu := range b.backlog {
		findOut(pu)
	}
}

type dependencies struct {
	sync.Mutex
	required  map[gomel.Hash]int
	neededFor map[gomel.Hash][]*gomel.Hash
	missing   []*gomel.Hash
}

func newDeps() *dependencies {
	return &dependencies{
		required:  make(map[gomel.Hash]int),
		neededFor: make(map[gomel.Hash][]*gomel.Hash),
	}
}

// add the given hashes as dependencies for h
func (d *dependencies) add(h *gomel.Hash, deps []*gomel.Hash) {
	d.Lock()
	defer d.Unlock()
	d.required[*h] = len(deps)
	for _, dh := range deps {
		neededFor := d.neededFor[*dh]
		if len(neededFor) == 0 {
			// this is the first time we need this hash
			d.missing = append(d.missing, dh)
		}
		d.neededFor[*dh] = append(neededFor, h)
	}
}

// scan which required units have been added to the dag.
// Returns the hashes of the satisfied dependencies.
func (d *dependencies) scan(dag gomel.Dag) []*gomel.Hash {
	d.Lock()
	defer d.Unlock()
	result := []*gomel.Hash{}
	units := dag.Get(d.missing)
	newMissing := make([]*gomel.Hash, 0, len(d.missing))
	for i, h := range d.missing {
		if units[i] != nil {
			result = append(result, h)
		} else {
			newMissing = append(newMissing, h)
		}
	}
	d.missing = newMissing
	return result
}

// satisfy the provided dependencies.
// Returns hashes of units that now have all their dependencies satisfied.
func (d *dependencies) satisfy(satisfiedHashes []*gomel.Hash) []*gomel.Hash {
	result := []*gomel.Hash{}
	for _, h := range satisfiedHashes {
		result = append(result, d.satisfyHash(h)...)
	}
	return result
}

func (d *dependencies) satisfyHash(h *gomel.Hash) []*gomel.Hash {
	d.Lock()
	defer d.Unlock()
	result := []*gomel.Hash{}
	for _, hh := range d.neededFor[*h] {
		d.required[*hh]--
		if d.required[*hh] == 0 {
			delete(d.required, *hh)
			result = append(result, hh)
		}
	}
	delete(d.neededFor, *h)
	return result
}
