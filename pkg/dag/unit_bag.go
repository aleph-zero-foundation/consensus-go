package dag

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unitBag struct {
	sync.RWMutex
	contents map[gomel.Hash]gomel.Unit
}

func newUnitBag() *unitBag {
	return &unitBag{contents: map[gomel.Hash]gomel.Unit{}}
}

func (units *unitBag) add(u gomel.Unit) {
	units.Lock()
	defer units.Unlock()
	units.contents[*u.Hash()] = u
}

func (units *unitBag) getOne(hash *gomel.Hash) gomel.Unit {
	units.RLock()
	defer units.RUnlock()
	return units.contents[*hash]
}

func (units *unitBag) getMany(hashes []*gomel.Hash) []gomel.Unit {
	units.RLock()
	defer units.RUnlock()
	result := make([]gomel.Unit, len(hashes))
	for i, h := range hashes {
		if h == nil {
			continue
		}
		if u, ok := units.contents[*h]; ok {
			result[i] = u
		}
	}
	return result
}

func (units *unitBag) getAll() []gomel.Unit {
	units.RLock()
	defer units.RUnlock()
	result := make([]gomel.Unit, len(units.contents))
	for _, u := range units.contents {
		result = append(result, u)
	}
	return result
}
