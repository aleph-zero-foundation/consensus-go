package growing

import (
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unit struct {
	creator       uint16
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
func (u *unit) Creator() uint16 {
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

func (u *unit) initialize(dag *Dag) {
	u.computeHeight()
	u.computeFloor(dag.nProcesses)
	u.computeLevel()
	u.computeForkingHeight(dag)
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

func (u *unit) computeFloor(nProcesses uint16) {
	// This version of the algorithm tries to minimize the number of heap allocations. It achieves this goal by means of
	// pre-allocating a continuous region of memory which is then used for storing all values of the computed floor (instead of
	// storing values of floor in separate slices for each process). At each index of the computed slice-of-slices we store a
	// slice that was created using a slice-expression pointing to that continuous storage. This way, assuming there are no
	// forks, it should only make two heap allocations in total, i.e. one for u.floor and one for storage variable floors.
	// Please notice that it uses the fact that the zero value for any slice, which is denoted by `nil`, is in fact a struct
	// containing a nil pointer and so the instruction `make([][]gomel.Unit, nProcesses)` pre-allocates memory for storing these
	// structs. Further assignment of values to each of floor's indexes simply copies values of structs pointing to our
	// pre-allocated storage. Previous version of this algorithm was allocating new heap objects for each index of floor. In
	// case of forks this version requires at worst O(lg(S/N)) allocations, where S is the total size of the computed floor
	// value and N is the number of processes.

	// WARNING: computed slice-of-slices is read-only. Any attempt of appending some value at any index can damage it.
	// This is due to the technique we used here - at each index of floor we store a slice pointing to some bigger storage, so
	// appending to such slice may overwrite values at indexes that follow the one we modified.

	// pre-allocate memory for storing values for each process
	floor := make([][]gomel.Unit, nProcesses)
	if len(u.parents) == 0 {
		floor[u.creator] = []gomel.Unit{u}
		u.floor = floor
		return
	}
	// pre-allocate memory for all values for all processes - 0 `len` allows us to use append for sake of simplicity
	floors := make([]gomel.Unit, 0, nProcesses)

	for pid := uint16(0); pid < nProcesses; pid++ {
		if pid == u.creator {
			floors = append(floors, u)
			continue
		}
		gomel.CombineParentsFloorsPerProc(u.parents, pid, &floors)
	}

	if len(floors) != cap(floors) {
		newFloors := make([]gomel.Unit, len(floors))
		copy(newFloors, floors)
		floors = newFloors
	}

	for lastIx, pid := uint16(0), uint16(0); pid < nProcesses; pid++ {
		ix := lastIx
		for int(ix) < len(floors) && floors[ix].Creator() == pid {
			ix++
		}
		floor[pid] = floors[lastIx:ix]
		lastIx = ix
	}
	u.floor = floor
}

func (u *unit) computeLevel() {
	if gomel.Dealing(u) {
		u.setLevel(0)
		return
	}

	nProcesses := uint16(len(u.Floor()))
	// compliant unit have parents in ascending order of level
	maxLevelParents := u.parents[len(u.parents)-1].Level()

	level := maxLevelParents
	nSeen := uint16(0)

	// we should consider our self predecessor
	// it assumes that this unit is not an evidence of self-forking
	if pred, err := gomel.Predecessor(u); err == nil && pred.Level() == maxLevelParents {
		nSeen++
	}
	creator := u.Creator()
	hasQuorum := IsQuorum(nProcesses, nSeen)
	for pid, vs := range u.Floor() {
		if uint16(pid) == creator {
			continue
		}

		for _, unit := range vs {
			if unit.Level() == maxLevelParents {
				nSeen++
				if IsQuorum(nProcesses, nSeen) {
					level = maxLevelParents + 1
					hasQuorum = true
				}
				break
			}
		}

		if hasQuorum || !IsQuorum(nProcesses, nSeen+(nProcesses-(uint16(pid)+1))) {
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
