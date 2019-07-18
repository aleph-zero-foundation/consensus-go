package growing

import (
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unit struct {
	creator       int
	height        int
	level         int
	forkingHeight int
	signature     gomel.Signature
	hash          gomel.Hash
	parents       []gomel.Unit
	floor         [][]gomel.Unit
	data          []byte
	rsData        []byte
}

func newUnit(pu gomel.Preunit) *unit {
	return &unit{
		creator: pu.Creator(),
		hash:    *pu.Hash(),
		data:    pu.Data(),
		rsData:  pu.RandomSourceData(),
	}
}

func (u *unit) Floor() [][]gomel.Unit {
	return u.floor
}

func (u *unit) RandomSourceData() []byte {
	return u.rsData
}

func (u *unit) Data() []byte {
	return u.data
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
		return len(combineParentsFloorsPerProc(u, creator)) > 1
	}
	return len(u.floor[creator]) > 1
}

func (u *unit) initialize(dag *Dag) {
	u.computeHeight()
	u.computeFloor(dag.nProcesses)
	u.computeLevel()
	u.computeForkingHeight(dag)
}

func (u *unit) addParent(parent gomel.Unit) {
	u.parents = append(u.parents, parent)
}

func (u *unit) setLevel(level int) {
	u.level = level
}

func (u *unit) computeHeight() {
	if gomel.Dealing(u) {
		u.height = 0
	} else {
		predecessor, _ := gomel.Predecessor(u)
		u.height = predecessor.Height() + 1
	}
}

func (u *unit) computeFloor(nProcesses int) {
	u.floor = make([][]gomel.Unit, nProcesses)
	u.floor[u.creator] = []gomel.Unit{u}

	for _, parent := range u.parents {
		pFloor := parent.Floor()
		for pid := 0; pid < nProcesses; pid++ {
			if pid == u.creator {
				continue
			}
			for _, w := range pFloor[pid] {
				found, ri := false, -1
				for k, v := range u.floor[pid] {
					if ok, _ := w.(*unit).aboveWithinProc(v.(*unit)); ok {
						found = true
						ri = k
						break
					}
					if ok, _ := w.(*unit).belowWithinProc(v.(*unit)); ok {
						found = true
					}
				}
				if !found {
					u.floor[pid] = append(u.floor[pid], w)
				}
				if ri >= 0 {
					u.floor[pid][ri] = w
				}
			}
		}
	}
}

func combineParentsFloorsPerProc(u gomel.Unit, pid int) []gomel.Unit {
	newFloor := []gomel.Unit{}

	for _, parent := range u.Parents() {
		for _, w := range parent.Floor()[pid] {
			found, ri := false, -1
			for k, v := range newFloor {
				if ok, _ := w.(*unit).aboveWithinProc(v.(*unit)); ok {
					found = true
					ri = k
					break
				}
				if ok, _ := w.(*unit).belowWithinProc(v.(*unit)); ok {
					found = true
				}
			}
			if !found {
				newFloor = append(newFloor, w)
			}

			if ri >= 0 {
				newFloor[ri] = w
			}
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
	// compliant unit have parents in ascending order of level
	maxLevelParents := u.parents[len(u.parents)-1].Level()

	level := maxLevelParents
	nSeen := 0

	// we should consider our self predecessor
	if pred, err := gomel.Predecessor(u); err == nil && pred.Level() == maxLevelParents {
		nSeen++
	}

	creator := u.Creator()
	for pid, vs := range u.floor {
		if pid == creator {
			continue
		}

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

func (u *unit) computeForkingHeight(dag *Dag) {
	// this implementation works as long as there is no race for writing/reading to dag.maxUnits, i.e.
	// as long as units created by one process are added atomically
	if gomel.Dealing(u) {
		if len(dag.MaximalUnitsPerProcess().Get(u.creator)) > 0 {
			//this is a forking dealing unit
			u.forkingHeight = -1
		} else {
			u.forkingHeight = math.MaxInt32
		}
		return
	}
	predTmp, _ := gomel.Predecessor(u)
	predecessor := predTmp.(*unit)
	found := false
	for _, v := range dag.MaximalUnitsPerProcess().Get(u.creator) {
		if v == predecessor {
			found = true
			break
		}
	}
	if found {
		u.forkingHeight = predecessor.forkingHeight
	} else {
		// there is already a unit that has 'predecessor' as a predecessor, hence u is a fork
		if predecessor.forkingHeight < predecessor.height {
			u.forkingHeight = predecessor.forkingHeight
		} else {
			u.forkingHeight = predecessor.height
		}
	}
}
