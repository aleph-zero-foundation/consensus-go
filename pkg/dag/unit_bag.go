package dag

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unitBag struct {
	sync.RWMutex
	contents map[gomel.Hash]*unit
}

func newUnitBag() *unitBag {
	return &unitBag{contents: map[gomel.Hash]*unit{}}
}

func (units *unitBag) add(u *unit) {
	units.Lock()
	defer units.Unlock()
	units.contents[*u.Hash()] = u
}

func (units *unitBag) get(hashes []*gomel.Hash) ([]gomel.Unit, int) {
	units.RLock()
	defer units.RUnlock()
	result := make([]gomel.Unit, len(hashes))
	unknown := 0
	for i, h := range hashes {
		if u, ok := units.contents[*h]; ok {
			result[i] = u
		} else {
			result[i] = nil
			unknown++
		}
	}
	return result, unknown
}
