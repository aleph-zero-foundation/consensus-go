package growing

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
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

func (units *unitBag) get(hashes []*gomel.Hash) []gomel.Unit {
	units.RLock()
	defer units.RUnlock()
	result := make([]gomel.Unit, len(hashes))
	for i, h := range hashes {
		if u, ok := units.contents[*h]; ok {
			result[i] = u
		} else {
			result[i] = nil
		}
	}
	return result
}
