package dag

import (
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// freeUnit is a unit that is not yet included in the dag.
// It performs some of the necessary computations (floor, level and height)
// lazily, on demand.
type freeUnit struct {
	nProc       uint16
	creator     uint16
	signature   gomel.Signature
	hash        gomel.Hash
	controlHash gomel.Hash
	parents     []gomel.Unit
	data        []byte
	rsData      []byte
	height      int
	level       int
	floor       [][]gomel.Unit
}

func newUnit(pu gomel.Preunit, parents []gomel.Unit, nProc uint16) *freeUnit {
	return &freeUnit{
		nProc:       nProc,
		creator:     pu.Creator(),
		signature:   pu.Signature(),
		hash:        *pu.Hash(),
		controlHash: *pu.ControlHash(),
		parents:     parents,
		data:        pu.Data(),
		rsData:      pu.RandomSourceData(),
		height:      -1,
		level:       -1,
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

func (u *freeUnit) ControlHash() *gomel.Hash {
	return &u.controlHash
}

func (u *freeUnit) Parents() []gomel.Unit {
	return u.parents
}

func (u *freeUnit) Height() int {
	if u.height == -1 {
		u.computeHeight()
	}
	return u.height
}

func (u *freeUnit) computeHeight() {
	if gomel.Dealing(u) {
		u.height = 0
	} else {
		u.height = gomel.Predecessor(u).Height() + 1
	}
}

func (u *freeUnit) Level() int {
	if u.level == -1 {
		u.computeLevel()
	}
	return u.level
}

func (u *freeUnit) computeLevel() {
	u.level = gomel.LevelFromParents(u.parents)
}

func (u *freeUnit) Floor() [][]gomel.Unit {
	if u.floor == nil {
		u.computeFloor()
	}
	return u.floor
}

func (u *freeUnit) computeFloor() {
	// This version of the algorithm tries to minimize the number of heap allocations. It achieves this goal by means of
	// preallocating a continuous region of memory which is then used for storing all values of the computed floor (instead of
	// storing values of floor in separate slices for each process). At each index of the computed slice-of-slices we store a
	// slice that was created using a slice-expression pointing to that continuous storage. This way, assuming there are no
	// forks, it should only make two heap allocations in total, i.e. one for u.floor and one for storage variable floors.
	// Please notice that it uses the fact that the zero value for any slice, which is denoted by `nil`, is in fact a struct
	// containing a nil pointer and so the instruction `make([][]gomel.Unit, nProcesses)` preallocates memory for storing these
	// structs. Further assignment of values to each of floor's indexes simply copies values of structs pointing to our
	// preallocated storage. Previous version of this algorithm was allocating new heap objects for each index of floor. In
	// case of forks this version requires at worst O(lg(S/N)) allocations, where S is the total size of the computed floor
	// value and N is the number of processes.

	// WARNING: computed slice-of-slices is read-only. Any attempt of appending some value at any index can damage it.
	// This is due to the technique we used here - at each index of floor we store a slice pointing to some bigger storage, so
	// appending to such slice may overwrite values at indexes that follow the one we modified.

	// preallocate memory for storing values for each process
	u.floor = make([][]gomel.Unit, u.nProc)
	if u.parents[u.creator] == nil {
		u.floor[u.creator] = []gomel.Unit{u}
		return
	}
	// preallocate memory for all values for all processes - 0 `len` allows us to use append for sake of simplicity
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

// unit is a unit that is already inside the dag, and has all its properties precomputed and cached.
// It uses forking heights to optimize Below calls.
type unit struct {
	creator       uint16
	height        int
	level         int
	signature     gomel.Signature
	hash          gomel.Hash
	controlHash   gomel.Hash
	parents       []gomel.Unit
	floor         [][]gomel.Unit
	data          []byte
	rsData        []byte
	forkingHeight int
}

func emplaced(u gomel.Unit, dag *dag) *unit {
	result := &unit{
		creator:     u.Creator(),
		height:      u.Height(),
		level:       u.Level(),
		signature:   u.Signature(),
		hash:        *u.Hash(),
		controlHash: *u.ControlHash(),
		parents:     u.Parents(),
		floor:       u.Floor(),
		data:        u.Data(),
		rsData:      u.RandomSourceData(),
	}
	result.fixSelfFloor()
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

func (u *unit) ControlHash() *gomel.Hash {
	return &u.controlHash
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

// fixSelfFloor replaces the self-reference in the floor with the correct one
func (u *unit) fixSelfFloor() {
	u.floor[u.Creator()] = []gomel.Unit{u}
}

func (u *unit) computeForkingHeight(dag *dag) {
	// this implementation works as long as there is no race for writing/reading to dag.maxUnits, i.e.
	// as long as units created by one process are added atomically
	if gomel.Dealing(u) {
		if len(dag.MaximalUnitsPerProcess().Get(u.creator)) > 0 {
			// this is a forking dealing unit
			u.forkingHeight = -1
		} else {
			u.forkingHeight = math.MaxInt32
		}
		return
	}
	predTmp := gomel.Predecessor(u)
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
