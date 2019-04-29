package growing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"sync"
)

type unit struct {
	creator       int
	height        int
	level         int
	forkingHeight int
	signature     gomel.Signature
	hash          gomel.Hash
	parents       []gomel.Unit
	floor         [][]*unit
}

func newUnit(pu gomel.Preunit) *unit {
	return &unit{
		creator: pu.Creator(),
		hash:    *pu.Hash(),
	}
}

// Returns the creator id of the unit.
func (u *unit) Creator() int {
	return u.creator
}

// Signature returns unit's signature.
func (u *unit) Signature() gomel.Signature {
	return u.signature
}

// Returns the hash of the unit.
func (u *unit) Hash() *gomel.Hash {
	return &u.hash
}

// How many units created by the same process are below the unit.
func (u *unit) Height() int {
	return u.height
}

// Returns the parents of the unit.
func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

// Returns the level of the unit.
func (u *unit) Level() int {
	return u.level
}

func (u *unit) HasForkingEvidence(creator int) bool {
	// using the knowledge of maximal units produced by 'creator' that are below some of the parents (their floor attributes),
	// check whether collection of these maximal units has a single maximal element
	if creator == u.creator {
		var floor []*unit
		for _, parent := range u.parents {
			actualParent := parent.(*unit)
			floor = append(floor, actualParent.floor[creator]...)
		}
		return len(combineFloorsPerProc(floor)) > 1
	} else {
		return len(u.floor[creator]) > 1
	}
}

func (u *unit) computeHeight() {
	if len(u.parents) == 0 {
		u.height = 0
	} else {
		u.height = u.Parents()[0].Height() + 1
	}
}

func (u *unit) addParent(parent gomel.Unit) {
	u.parents = append(u.parents, parent)
}

func (u *unit) setLevel(level int) {
	u.level = level
}

func (u *unit) computeFloor(nProcesses int) {
	u.floor = make([][]*unit, nProcesses, nProcesses)
	u.floor[u.creator] = []*unit{u}

	floors := make([][]*unit, nProcesses, nProcesses)

	for _, parent := range u.parents {
		if realParent, ok := parent.(*unit); ok {
			for pid := 0; pid < nProcesses; pid++ {
				floors[pid] = append(floors[pid], realParent.floor[pid]...)
			}
		} else {
			// TODO: this might be needed in the far future when there are special units that separate existing and nonexistent units
		}
	}

	var wg sync.WaitGroup
	for pid := 0; pid < nProcesses; pid++ {
		if pid == u.creator {
			continue
		}
		pid := pid
		wg.Add(1)
		go func() {
			defer wg.Done()
			u.floor[pid] = combineFloorsPerProc(floors[pid])
		}()
	}

	wg.Wait()
}

func combineFloorsPerProc(floors []*unit) []*unit {
	newFloor := []*unit{}

	// Computes maximal elements in floors and stores them in newFloor
	// floors contains elements created by only one proc
	if len(floors) == 0 {
		return newFloor
	}

	for _, u := range floors {
		found, ri := false, -1
		for k, v := range newFloor {
			if ok, _ := u.aboveWithinProc(v); ok {
				found = true
				ri = k
				break
			}
			if ok, _ := u.belowWithinProc(v); ok {
				found = true
			}
		}
		if !found {
			newFloor = append(newFloor, u)
		}

		if ri >= 0 {
			newFloor[ri] = u
		}
	}

	return newFloor
}
