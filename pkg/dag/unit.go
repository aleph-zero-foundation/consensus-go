package dag

import (
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type freeUnit struct {
	nProc      uint16
	creator    uint16
	signature  gomel.Signature
	hash       gomel.Hash
	parents    []gomel.Unit
	data       []byte
	rsData     []byte
	height     int
	heightInit bool
	level      int
	levelInit  bool
	floor      [][]gomel.Unit
	floorInit  bool
}

func newUnit(pu gomel.Preunit, parents []gomel.Unit, nProc uint16) *freeUnit {
	return &freeUnit{
		nProc:   nProc,
		creator: pu.Creator(),
		hash:    *pu.Hash(),
		data:    pu.Data(),
		rsData:  pu.RandomSourceData(),
		parents: parents,
	}
}

func (u *freeUnit) RandomSourceData() []byte {
	return u.rsData
}

func (u *freeUnit) Data() []byte {
	return u.data
}

func (u *freeUnit) Creator() uint16 {
	return u.creator
}

func (u *freeUnit) Signature() gomel.Signature {
	return u.signature
}

func (u *freeUnit) Hash() *gomel.Hash {
	return &u.hash
}

func (u *freeUnit) Parents() []gomel.Unit {
	return u.parents
}

func (u *freeUnit) Height() int {
	if !u.heightInit {
		u.computeHeight()
	}
	return u.height
}

func (u *freeUnit) computeHeight() {
	if gomel.Dealing(u) {
		u.height = 0
	} else {
		predecessor, _ := gomel.Predecessor(u)
		u.height = predecessor.Height() + 1
	}
	u.heightInit = true
}

func (u *freeUnit) Level() int {
	if !u.levelInit {
		u.computeLevel()
	}
	return u.level
}

func (u *freeUnit) computeLevel() {
	if gomel.Dealing(u) {
		u.level = 0
		u.levelInit = true
		return
	}

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
	hasQuorum := IsQuorum(u.nProc, nSeen)
	for pid, vs := range u.Floor() {
		pid := uint16(pid)
		if pid == creator {
			continue
		}

		for _, unit := range vs {
			if unit.Level() == maxLevelParents {
				nSeen++
				if IsQuorum(u.nProc, nSeen) {
					level = maxLevelParents + 1
					hasQuorum = true
				}
				break
			}
		}

		if hasQuorum || !IsQuorum(u.nProc, nSeen+(u.nProc-(pid+1))) {
			break
		}
	}
	u.level = level
	u.levelInit = true
}

func (u *freeUnit) Floor() [][]gomel.Unit {
	if !u.floorInit {
		u.computeFloor()
	}
	return u.floor
}

func (u *freeUnit) computeFloor() {
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
	u.floor = make([][]gomel.Unit, u.nProc)
	if len(u.parents) == 0 {
		u.floor[u.creator] = []gomel.Unit{u}
		return
	}
	// pre-allocate memory for all values for all processes - 0 `len` allows us to use append for sake of simplicity
	floors := make([]gomel.Unit, 0, u.nProc)

	for pid := uint16(0); pid < u.nProc; pid++ {
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

	for lastIx, pid := uint16(0), uint16(0); pid < u.nProc; pid++ {
		ix := lastIx
		for int(ix) < len(floors) && floors[ix].Creator() == pid {
			ix++
		}
		u.floor[pid] = floors[lastIx:ix]
		lastIx = ix
	}
}

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

func emplaced(u gomel.Unit, dag *dag) *unit {
	result := &unit{
		creator:   u.Creator(),
		height:    u.Height(),
		level:     u.Level(),
		signature: u.Signature(),
		hash:      *u.Hash(),
		parents:   u.Parents(),
		floor:     u.Floor(),
		data:      u.Data(),
		rsData:    u.RandomSourceData(),
	}
	result.computeForkingHeight(dag)
	return result
}

func (u *unit) RandomSourceData() []byte {
	return u.rsData
}

func (u *unit) Data() []byte {
	return u.data
}

func (u *unit) Creator() uint16 {
	return u.creator
}

func (u *unit) Signature() gomel.Signature {
	return u.signature
}

func (u *unit) Hash() *gomel.Hash {
	return &u.hash
}

func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

func (u *unit) Height() int {
	return u.height
}

func (u *unit) Floor() [][]gomel.Unit {
	return u.floor
}

func (u *unit) Level() int {
	return u.level
}

func (u *unit) computeForkingHeight(dag *dag) {
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
