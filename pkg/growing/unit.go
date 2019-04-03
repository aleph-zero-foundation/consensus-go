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

func (u *unit) setHeight(height int) {
	u.height = height
}

func (u *unit) addParent(parent gomel.Unit) {
	u.parents = append(u.parents, parent)
}

func (u *unit) setLevel(level int) {
	u.level = level
}

func (u *unit) computeFloor(nProcesses int) {
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

	u.floor = make([][]*unit, nProcesses, nProcesses)
	var wg sync.WaitGroup
	for pid := 0; pid < nProcesses; pid++ {
		pid := pid
		wg.Add(1)
		go func() {
			defer wg.Done()
			combineFloorsPerProc(floors[pid], u.floor[pid])
		}()
	}

	wg.Wait()
}

func combineFloorsPerProc(floors []*unit, newFloor []*unit) {
	// Computes maximal elements in floors and stores them in newFloor
	// floors contains elements created by only one proc
	if len(floors) == 0 {
		return
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
}

//====================================================================================
//                                 ORDERING
//====================================================================================

func (u *unit) belowWithinProc(v *unit) (bool, error) {
	if u.creator != v.creator {
		return false, gomel.NewDataError("Different creators")
	}
	if u.height > v.height {
		return false, nil
	}

	// if u is below the pid's forking height then there is a path from v to u
	if u.height <= v.forkingHeight {
		return true, nil
	}

	// in forking situation we have to check if going down from v to u.height we find u
	w := v
	for w.height > u.height {
		wVal, err := gomel.Predecessor(u)
		if err != nil {
			return false, err
		}

		w = wVal.(*unit)
	}

	return u == w, nil
}

func (u *unit) aboveWithinProc(v *unit) (bool, error) {
	return v.belowWithinProc(u)
}

// Below checks if a unit u is less than a unit v
func (u *unit) Below(v gomel.Unit) bool {
	var V *unit
	var ok bool
	if V, ok = v.(*unit); !ok {
		// TODO: this might be needed in the far future when there are special units that separate existing and nonexistent units
	}
	for _, w := range V.floor[u.creator] {
		if ok, _ := u.belowWithinProc(w); ok {
			return true
		}
	}
	return false
}

// Above checks if a unit u is greater than a unit v
func (u *unit) Above(v gomel.Unit) bool {
	return v.Below(u)
}
