package dag

import (
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type freeUnit struct {
	nProc     uint16
	creator   uint16
	signature gomel.Signature
	hash      gomel.Hash
	parents   []gomel.Unit
	crown     gomel.Crown
	data      gomel.Data
	rsData    []byte
	height    int
	level     int
	floor     [][]gomel.Unit
}

// NewUnit that is not yet included in the dag.
// It performs some of the necessary computations (floor, level and height)
// lazily, on demand.
func NewUnit(pu gomel.Preunit, parents []gomel.Unit) gomel.Unit {
	return &freeUnit{
		nProc:     uint16(len(parents)),
		creator:   pu.Creator(),
		signature: pu.Signature(),
		crown:     *pu.View(),
		hash:      *pu.Hash(),
		parents:   parents,
		data:      pu.Data(),
		rsData:    pu.RandomSourceData(),
		height:    -1,
		level:     -1,
	}
}

func (u *freeUnit) RandomSourceData() []byte {
	return u.rsData
}

func (u *freeUnit) Data() gomel.Data {
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

func (u *freeUnit) View() *gomel.Crown {
	return &u.crown
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

// unitInDag is a unit that is already inside the dag, and has all its properties precomputed and cached.
// It uses forking heights to optimize Above calls.
type unitInDag struct {
	gomel.Unit
	forkingHeight int
}

func prepared(u gomel.Unit, dag *dag) *unitInDag {
	result := &unitInDag{u, 0}
	result.fixSelfFloor()
	result.computeForkingHeight(dag)
	return result
}

// fixSelfFloor replaces the self-reference in the floor with the correct one
func (u *unitInDag) fixSelfFloor() {
	//floor := u.Floor() TODO!! CHECK THIS
	u.Floor()[u.Creator()] = []gomel.Unit{u}
}

func (u *unitInDag) computeForkingHeight(dag *dag) {
	// this implementation works as long as there is no race for writing/reading to dag.maxUnits, i.e.
	// as long as units created by one process are added atomically
	if gomel.Dealing(u) {
		if len(dag.MaximalUnitsPerProcess().Get(u.Creator())) > 0 {
			// this is a forking dealing unit
			u.forkingHeight = -1
		} else {
			u.forkingHeight = math.MaxInt32
		}
		return
	}
	predTmp := gomel.Predecessor(u)
	predecessor := predTmp.(*unitInDag)
	found := false
	for _, v := range dag.MaximalUnitsPerProcess().Get(u.Creator()) {
		if v == predecessor {
			found = true
			break
		}
	}
	if found {
		u.forkingHeight = predecessor.forkingHeight
	} else {
		// there is already a unit that has 'predecessor' as a predecessor, hence u is a fork
		if predecessor.forkingHeight < predecessor.Height() {
			u.forkingHeight = predecessor.forkingHeight
		} else {
			u.forkingHeight = predecessor.Height()
		}
	}
}
