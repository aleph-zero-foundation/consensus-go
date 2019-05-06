package growing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"math"
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

func (u *unit) initialize(poset *Poset) {
	u.computeHeight()
	u.computeFloor(poset.nProcesses)
	u.computeLevel()
	u.computeForkingHeight(poset)
}

func (u *unit) addParent(parent gomel.Unit) {
	u.parents = append(u.parents, parent)
}

func (u *unit) setLevel(level int) {
	u.level = level
}

func (u *unit) computeHeight() {
	if len(u.parents) == 0 {
		u.height = 0
	} else {
		u.height = u.Parents()[0].Height() + 1
	}
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
	wg.Add(nProcesses - 1)

	for pid := 0; pid < nProcesses; pid++ {
		if pid == u.creator {
			continue
		}
		go func(pid int) {
			defer wg.Done()
			u.floor[pid] = combineFloorsPerProc(floors[pid])
		}(pid)
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

func (u *unit) computeLevel() {
	if gomel.Dealing(u) {
		u.setLevel(0)
		return
	}

	nProcesses := len(u.floor)
	maxLevelParents := 0
	for _, w := range u.parents {
		wLevel := w.Level()
		if wLevel > maxLevelParents {
			maxLevelParents = wLevel
		}
	}

	level := maxLevelParents
	nSeen := 0
	for pid, vs := range u.floor {

		for _, unit := range vs {
			if unit.Level() == maxLevelParents {
				nSeen++
				break
			}
		}

		// optimization to not loop over all processes if quorum cannot be reached anyway
		if !IsQuorum(nProcesses, nSeen+(nProcesses-(pid+1))) {
			break
		}

		if IsQuorum(nProcesses, nSeen) {
			level = maxLevelParents + 1
			break
		}
	}
	u.setLevel(level)
}

func (u *unit) computeForkingHeight(p *Poset) {
	// this implementation works as long as there is no race for writing/reading to p.maxUnits, i.e.
	// as long as units created by one process are added atomically
	if len(u.parents) == 0 {
		if len(p.MaximalUnitsPerProcess().Get(u.creator)) > 0 {
			//this is a forking dealing unit
			u.forkingHeight = -1
		} else {
			u.forkingHeight = math.MaxInt32
		}
		return
	}
	up := u.parents[0].(*unit)
	found := false
	for _, v := range p.MaximalUnitsPerProcess().Get(u.creator) {
		if v == up {
			found = true
			break
		}
	}
	if found {
		u.forkingHeight = up.forkingHeight
	} else {
		// there is already a unit that has up as a predecessor, hence u is a fork
		if up.forkingHeight < up.height {
			u.forkingHeight = up.forkingHeight
		} else {
			u.forkingHeight = up.height
		}
	}
}
