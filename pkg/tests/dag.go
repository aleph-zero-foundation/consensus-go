package tests

import (
	"sort"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Dag is a basic implementation of dag for testing
type Dag struct {
	sync.RWMutex
	nProcesses int
	primeUnits []gomel.SlottedUnits
	// maximalHeight is the maximalHeight of a unit created per process
	maximalHeight []int
	unitsByHeight []gomel.SlottedUnits
	unitByHash    map[gomel.Hash]gomel.Unit
}

func newDag(dagConfiguration gomel.DagConfig) *Dag {
	n := dagConfiguration.NProc()
	maxHeight := make([]int, n)
	for pid := 0; pid < n; pid++ {
		maxHeight[pid] = -1
	}
	newDag := &Dag{
		nProcesses:    n,
		primeUnits:    []gomel.SlottedUnits{},
		unitsByHeight: []gomel.SlottedUnits{},
		maximalHeight: maxHeight,
		unitByHash:    make(map[gomel.Hash]gomel.Unit),
	}
	return newDag
}

// AddUnit adds a unit in a thread safe manner without trying to be clever.
func (dag *Dag) AddUnit(pu gomel.Preunit, rs gomel.RandomSource, callback gomel.Callback) {
	var u unit
	err := dehashParents(&u, dag, pu)
	if err != nil {
		callback(pu, nil, err)
		return
	}
	// Setting height, creator, signature, version, hash
	setBasicInfo(&u, dag, pu)
	setLevel(&u, dag)
	setFloor(&u, dag)

	//Setting dag variables
	updateDag(&u, dag)
	callback(pu, &u, nil)
}

// PrimeUnits returns the prime units at the given level.
func (dag *Dag) PrimeUnits(level int) gomel.SlottedUnits {
	dag.RLock()
	defer dag.RUnlock()
	if level < len(dag.primeUnits) {
		return dag.primeUnits[level]
	}
	return nil
}

// MaximalUnitsPerProcess returns the maximal units for all processes.
func (dag *Dag) MaximalUnitsPerProcess() gomel.SlottedUnits {
	dag.RLock()
	defer dag.RUnlock()
	su := newSlottedUnits(dag.nProcesses)
	for pid := 0; pid < dag.nProcesses; pid++ {
		if dag.maximalHeight[pid] >= 0 {
			su.Set(pid, dag.unitsByHeight[dag.maximalHeight[pid]].Get(pid))
		}
	}
	return su
}

// Get returns the units with the given hashes or nil, when it doesn't find them.
func (dag *Dag) Get(hashes []*gomel.Hash) []gomel.Unit {
	dag.RLock()
	defer dag.RUnlock()
	result := make([]gomel.Unit, len(hashes))
	for i, h := range hashes {
		result[i] = dag.unitByHash[*h]
	}
	return result
}

// NProc returns the number of processes in this dag.
func (dag *Dag) NProc() int {
	// nProcesses doesn't change so no lock needed
	return dag.nProcesses
}

// IsQuorum checks whether the provided number of processes constitutes a quorum.
func (dag *Dag) IsQuorum(number int) bool {
	// nProcesses doesn't change so no lock needed
	return 3*number >= 2*dag.nProcesses
}

func dehashParents(u *unit, dag *Dag, pu gomel.Preunit) error {
	dag.RLock()
	defer dag.RUnlock()
	u.parents = []gomel.Unit{}
	unknown := 0
	for _, parentHash := range pu.Parents() {
		if _, ok := dag.unitByHash[*parentHash]; !ok {
			unknown++
		} else {
			u.parents = append(u.parents, dag.unitByHash[*parentHash])
		}
	}
	if unknown > 0 {
		return gomel.NewUnknownParents(unknown)
	}
	return nil
}

func setBasicInfo(u *unit, dag *Dag, pu gomel.Preunit) {
	dag.RLock()
	defer dag.RUnlock()
	u.creator = pu.Creator()
	if len(u.parents) == 0 {
		u.height = 0
	} else {
		u.height = u.parents[0].Height() + 1
	}
	u.signature = pu.Signature()
	u.hash = *pu.Hash()
	u.data = pu.Data()
	u.rsData = pu.RandomSourceData()
	if len(dag.unitsByHeight) <= u.height {
		u.version = 0
	} else {
		u.version = len(dag.unitsByHeight[u.height].Get(u.creator))
	}
}

func updateDag(u *unit, dag *Dag) {
	dag.Lock()
	defer dag.Unlock()

	if u.Height() == 0 {
		if len(dag.unitsByHeight) == 0 {
			dag.unitsByHeight = append(dag.unitsByHeight, newSlottedUnits(dag.nProcesses))
		}
		dag.unitsByHeight[0].Set(u.Creator(), append(dag.unitsByHeight[0].Get(u.Creator()), u))
		if len(dag.primeUnits) == 0 {
			dag.primeUnits = append(dag.primeUnits, newSlottedUnits(dag.nProcesses))
		}
		dag.primeUnits[0].Set(u.Creator(), append(dag.primeUnits[0].Get(u.Creator()), u))
	} else {
		if gomel.Prime(u) {
			if len(dag.primeUnits) <= u.Level() {
				dag.primeUnits = append(dag.primeUnits, newSlottedUnits(dag.nProcesses))
			}
			dag.primeUnits[u.Level()].Set(u.Creator(), append(dag.primeUnits[u.Level()].Get(u.Creator()), u))
		}
		if len(dag.unitsByHeight) <= u.Height() {
			dag.unitsByHeight = append(dag.unitsByHeight, newSlottedUnits(dag.nProcesses))
		}
		dag.unitsByHeight[u.Height()].Set(u.Creator(), append(dag.unitsByHeight[u.Height()].Get(u.Creator()), u))
	}
	if u.Height() > dag.maximalHeight[u.Creator()] {
		dag.maximalHeight[u.Creator()] = u.Height()
	}
	dag.unitByHash[*u.Hash()] = u
}

func setFloor(u *unit, dag *Dag) {
	dag.RLock()
	defer dag.RUnlock()
	parentsFloorUnion := make([][]gomel.Unit, dag.NProc())
	parentsFloorUnion[u.Creator()] = []gomel.Unit{u}
	for _, v := range u.Parents() {
		for pid, units := range v.Floor() {
			parentsFloorUnion[pid] = append(parentsFloorUnion[pid], units...)
		}
	}
	result := make([][]gomel.Unit, dag.NProc())
	for pid := 0; pid < dag.NProc(); pid++ {
		sort.Slice(parentsFloorUnion[pid], func(i, j int) bool {
			return parentsFloorUnion[pid][i].Height() > parentsFloorUnion[pid][j].Height()
		})
		for _, v := range parentsFloorUnion[pid] {
			ok := true
			for _, f := range result[pid] {
				if v.Below(f) {
					ok = false
					break
				}
			}
			if ok {
				result[pid] = append(result[pid], v)
			}
		}
	}
	u.floor = result
}

func setLevel(u *unit, dag *Dag) {
	dag.RLock()
	defer dag.RUnlock()
	if u.Height() == 0 {
		u.level = 0
		return
	}
	maxLevelBelow := -1
	for _, up := range u.Parents() {
		if up.Level() > maxLevelBelow {
			maxLevelBelow = up.Level()
		}
	}
	u.level = maxLevelBelow
	seenProcesses := make(map[int]bool)
	seenUnits := make(map[gomel.Hash]bool)
	seenUnits[*u.Hash()] = true
	queue := []gomel.Unit{}
	queue = append(queue, u.Parents()...)
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if w.Level() == maxLevelBelow {
			seenUnits[*w.Hash()] = true
			seenProcesses[w.Creator()] = true
			for _, wParent := range w.Parents() {
				if _, exists := seenUnits[*wParent.Hash()]; !exists {
					queue = append(queue, wParent)
					seenUnits[*wParent.Hash()] = true
				}
			}
		}
	}
	if dag.IsQuorum(len(seenProcesses)) {
		u.level = maxLevelBelow + 1
	}
}

func (dag *Dag) getPrimeUnitsOnLevel(level int) []gomel.Unit {
	result := []gomel.Unit{}
	for pid := 0; pid < dag.NProc(); pid++ {
		result = append(result, dag.primeUnits[level].Get(pid)...)
	}
	return result
}
