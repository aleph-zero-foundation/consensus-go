// Package tests implements a very simple and unoptimized version of the dag.
//
// It also contains mocks of various other structures that are useful for tests.
// Additionally, there is a mechanism for saving and loading dags from files.
package tests

import (
	"errors"
	"sort"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Dag is a basic implementation of dag for testing.
type Dag struct {
	sync.RWMutex
	nProcesses uint16
	primeUnits []gomel.SlottedUnits
	// maximalHeight is the maximalHeight of a unit created per process
	maximalHeight []int
	unitsByHeight []gomel.SlottedUnits
	unitByHash    map[gomel.Hash]gomel.Unit
}

func newDag(dagConfiguration config.Dag) *Dag {
	n := dagConfiguration.NProc()
	maxHeight := make([]int, n)
	for pid := uint16(0); pid < n; pid++ {
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

// Decode the given preunit to a unit.
func (dag *Dag) Decode(pu gomel.Preunit) (gomel.Unit, error) {
	var u unit
	err := dehashParents(&u, dag, pu)
	if err != nil {
		return nil, err
	}
	// Setting height, creator, signature, version, hash
	setBasicInfo(&u, dag, pu)
	setLevel(&u, dag)
	setFloor(&u, dag)
	return &u, nil
}

// Check accepts everything.
func (dag *Dag) Check(u gomel.Unit) error {
	return nil
}

// Emplace the unit in the dag.
func (dag *Dag) Emplace(u gomel.Unit) gomel.Unit {
	updateDag(u, dag)
	return u
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

// Get returns the units with the given hashes or nil, when it doesn't find them.
func (dag *Dag) Get(hashes []*gomel.Hash) []gomel.Unit {
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

func tryAllSubsets(units [][]gomel.Unit, pu gomel.Preunit) ([]gomel.Unit, error) {
	nProc := len(units)
	answer := make([]gomel.Unit, nProc)
	hashes := make([]*gomel.Hash, nProc)
	var rec func(int) bool
	rec = func(ind int) bool {
		if ind == nProc {
			if *pu.ControlHash() == *gomel.CombineHashes(hashes) {
				return true
			}
			return false
		}
		if pu.ParentsHeights()[ind] == -1 {
			return rec(ind + 1)
		}
		for _, u := range units[ind] {
			answer[ind] = u
			hashes[ind] = u.Hash()
			if rec(ind + 1) {
				return true
			}
			hashes[ind] = nil
		}
		return false
	}
	if rec(0) {
		return answer, nil
	}
	return nil, errors.New("wrong control hash")
}

func dehashParents(u *unit, dag *Dag, pu gomel.Preunit) error {
	dag.RLock()
	defer dag.RUnlock()
	possibleParents := make([][]gomel.Unit, dag.NProc())
	unknown := 0
	for i, parentHeight := range pu.ParentsHeights() {
		if parentHeight == -1 {
			continue
		}
		su := dag.unitsByHeight[parentHeight]
		possibleParents[i] = su.Get(uint16(i))
		if possibleParents[i] == nil {
			unknown++
		}
	}
	if unknown > 0 {
		return gomel.NewUnknownParents(unknown)
	}
	parents, err := tryAllSubsets(possibleParents, pu)
	if err != nil {
		return err
	}
	u.parents = parents
	return nil
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
	u.controlHash = *pu.ControlHash()
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
		if v == nil {
			continue
		}
		for pid, units := range v.Floor() {
			parentsFloorUnion[pid] = append(parentsFloorUnion[pid], units...)
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
	u.level = gomel.LevelFromParents(u.parents)
}

func (dag *Dag) getPrimeUnitsOnLevel(level int) []gomel.Unit {
	result := []gomel.Unit{}
	for pid := uint16(0); pid < dag.NProc(); pid++ {
		result = append(result, dag.primeUnits[level].Get(pid)...)
	}
	return result
}
