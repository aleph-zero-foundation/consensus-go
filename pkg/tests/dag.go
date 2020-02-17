// Package tests implements a very simple and unoptimized version of the dag.
//
// It also contains mocks of various other structures that are useful for tests.
// Additionally, there is a mechanism for saving and loading dags from files.
package tests

import (
	"sort"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Dag is a basic implementation of dag for testing.
type Dag struct {
	sync.RWMutex
	nProcesses uint16
	epochID    gomel.EpochID
	primeUnits []gomel.SlottedUnits
	// maximalHeight is the maximalHeight of a unit created per process
	maximalHeight []int
	unitsByHeight []gomel.SlottedUnits
	unitByHash    map[gomel.Hash]gomel.Unit
	checks        []gomel.UnitChecker
	transforms    []gomel.UnitTransformer
	preInsert     []gomel.InsertHook
	postInsert    []gomel.InsertHook
}

func newDag(nProc uint16) *Dag {
	maxHeight := make([]int, nProc)
	for pid := uint16(0); pid < nProc; pid++ {
		maxHeight[pid] = -1
	}
	newDag := &Dag{
		nProcesses:    nProc,
		primeUnits:    []gomel.SlottedUnits{},
		unitsByHeight: []gomel.SlottedUnits{},
		maximalHeight: maxHeight,
		unitByHash:    make(map[gomel.Hash]gomel.Unit),
	}
	return newDag
}

// EpochID implementation
func (dag *Dag) EpochID() gomel.EpochID {
	return dag.epochID
}

// AddCheck implementation
func (dag *Dag) AddCheck(check gomel.UnitChecker) {
	dag.checks = append(dag.checks, check)
}

// AddTransform implementation
func (dag *Dag) AddTransform(trans gomel.UnitTransformer) {
	dag.transforms = append(dag.transforms, trans)
}

// BeforeInsert implementation
func (dag *Dag) BeforeInsert(hook gomel.InsertHook) {
	dag.preInsert = append(dag.preInsert, hook)
}

// AfterInsert implementation
func (dag *Dag) AfterInsert(hook gomel.InsertHook) {
	dag.postInsert = append(dag.postInsert, hook)
}

// DecodeParents of the given preunit.
func (dag *Dag) DecodeParents(pu gomel.Preunit) ([]gomel.Unit, error) {
	heights := pu.View().Heights
	parents := make([]gomel.Unit, len(heights))
	unknown := 0
	dag.RLock()
	defer dag.RUnlock()
	for i, h := range heights {
		if h == -1 {
			continue
		}
		su := dag.UnitsOnHeight(h)
		if su == nil {
			unknown++
			continue
		}
		units := su.Get(uint16(i))
		if len(units) == 0 {
			unknown++
			continue
		}
		parents[i] = units[0]
	}

	if unknown > 0 {
		return nil, gomel.NewUnknownParents(unknown)
	}
	return parents, nil
}

// BuildUnit makes a new test unit based on the given preunit and parents.
func (dag *Dag) BuildUnit(pu gomel.Preunit, parents []gomel.Unit) gomel.Unit {
	var u unit
	u.parents = parents
	// Setting height, creator, signature, version, hash
	setBasicInfo(&u, dag, pu)
	setLevel(&u, dag)
	setFloor(&u, dag)
	return &u
}

// Check checks.
func (dag *Dag) Check(u gomel.Unit) error {
	for _, check := range dag.checks {
		if err := check(u); err != nil {
			return err
		}
	}
	return nil
}

// Transform transforms.
func (dag *Dag) Transform(u gomel.Unit) gomel.Unit {
	for _, trans := range dag.transforms {
		u = trans(u)
	}
	return u
}

// Insert the unit into the dag.
func (dag *Dag) Insert(u gomel.Unit) {
	for _, hook := range dag.preInsert {
		hook(u)
	}
	updateDag(u, dag)
	for _, hook := range dag.postInsert {
		hook(u)
	}
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

// UnitsOnHeight returns the units on the given height.
func (dag *Dag) UnitsOnHeight(height int) gomel.SlottedUnits {
	dag.RLock()
	defer dag.RUnlock()
	if height < len(dag.unitsByHeight) {
		return dag.unitsByHeight[height]
	}
	return nil
}

// MaximalUnitsPerProcess returns the maximal units for all processes.
func (dag *Dag) MaximalUnitsPerProcess() gomel.SlottedUnits {
	dag.RLock()
	defer dag.RUnlock()
	su := newSlottedUnits(dag.nProcesses)
	for pid := uint16(0); pid < dag.nProcesses; pid++ {
		if dag.maximalHeight[pid] >= 0 {
			su.Set(pid, dag.unitsByHeight[dag.maximalHeight[pid]].Get(pid))
		}
	}
	return su
}

// GetUnit returns the units with the given hashes or nil, when it doesn't find them.
func (dag *Dag) GetUnit(hash *gomel.Hash) gomel.Unit {
	dag.RLock()
	defer dag.RUnlock()
	if hash != nil {
		return dag.unitByHash[*hash]
	}
	return nil
}

// GetUnits returns the units with the given hashes or nil, when it doesn't find them.
func (dag *Dag) GetUnits(hashes []*gomel.Hash) []gomel.Unit {
	dag.RLock()
	defer dag.RUnlock()
	result := make([]gomel.Unit, len(hashes))
	for i, h := range hashes {
		if h != nil {
			result[i] = dag.unitByHash[*h]
		}
	}
	return result
}

// GetByID returns all the units associated with the given ID.
func (dag *Dag) GetByID(id uint64) []gomel.Unit {
	height, creator, _ := gomel.DecodeID(id)
	dag.RLock()
	defer dag.RUnlock()
	if height >= len(dag.unitsByHeight) {
		return nil
	}
	return dag.unitsByHeight[height].Get(creator)
}

// NProc returns the number of processes in this dag.
func (dag *Dag) NProc() uint16 {
	// nProcesses doesn't change so no lock needed
	return dag.nProcesses
}

// IsQuorum checks whether the provided number of processes constitutes a quorum.
func (dag *Dag) IsQuorum(number uint16) bool {
	// nProcesses doesn't change so no lock needed
	return 3*number >= 2*dag.nProcesses
}

func setBasicInfo(u *unit, dag *Dag, pu gomel.Preunit) {
	dag.RLock()
	defer dag.RUnlock()
	u.creator = pu.Creator()
	if gomel.Predecessor(u) == nil {
		u.height = 0
	} else {
		u.height = gomel.Predecessor(u).Height() + 1
	}
	u.signature = pu.Signature()
	u.hash = *pu.Hash()
	u.crown = *pu.View()
	u.data = pu.Data()
	u.rsData = pu.RandomSourceData()
	if len(dag.unitsByHeight) <= u.height {
		u.version = 0
	} else {
		u.version = len(dag.unitsByHeight[u.height].Get(u.creator))
	}
}

func updateDag(u gomel.Unit, dag *Dag) {
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
		if len(dag.primeUnits) <= u.Level() {
			dag.primeUnits = append(dag.primeUnits, newSlottedUnits(dag.nProcesses))
		}
		dag.primeUnits[u.Level()].Set(u.Creator(), append(dag.primeUnits[u.Level()].Get(u.Creator()), u))
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
	for _, v := range u.Parents() {
		if v == nil {
			continue
		}
		parentsFloorUnion[v.Creator()] = append(parentsFloorUnion[v.Creator()], v)
		for pid := uint16(0); pid < dag.NProc(); pid++ {
			parentsFloorUnion[pid] = append(parentsFloorUnion[pid], v.Floor(pid)...)
		}
	}
	result := make([][]gomel.Unit, dag.NProc())
	for pid := uint16(0); pid < dag.NProc(); pid++ {
		sort.Slice(parentsFloorUnion[pid], func(i, j int) bool {
			return parentsFloorUnion[pid][i].Height() > parentsFloorUnion[pid][j].Height()
		})
		for _, v := range parentsFloorUnion[pid] {
			ok := true
			for _, f := range result[pid] {
				if f.AboveWithinProc(v) {
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
	u.level = gomel.LevelFromParents(u.parents)
}

func (dag *Dag) getPrimeUnitsOnLevel(level int) []gomel.Unit {
	result := []gomel.Unit{}
	for pid := uint16(0); pid < dag.NProc(); pid++ {
		result = append(result, dag.primeUnits[level].Get(pid)...)
	}
	return result
}
