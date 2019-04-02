package growing

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type unitBag struct {
	sync.RWMutex
}

func (units *unitBag) add(u *unit) {
	units.Lock()
	defer units.Unlock()
	// TODO: implement
}

func (units *unitBag) get(h gomel.Hash) (*unit, bool) {
	// TODO: implement
	return nil, false
}

func (units *unitBag) dehashParents(ub *unitBuilt) error {
	units.RLock()
	defer units.RUnlock()
	if _, ok := units.get(*ub.preunit.Hash()); ok {
		return &gomel.DuplicateUnit{}
	}
	for _, h := range ub.preunit.Parents() {
		parent, ok := units.get(h)
		if !ok {
			return gomel.NewDataError("Missing parent")
		}
		ub.result.addParent(parent)
	}
	return nil
}
