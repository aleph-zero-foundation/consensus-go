package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"sync"
)

// poset is a basic implementation of poset for testing
type poset struct {
	sync.RWMutex
	nProcesses int
	primeUnits []gomel.SlottedUnits
	// maximalHeight is the maximalHeight of a unit created per process
	maximalHeight []int
	unitsByHeight []gomel.SlottedUnits
	unitByHash    map[gomel.Hash]gomel.Unit
}

func newPoset(posetConfiguration gomel.PosetConfig) *poset {
	n := posetConfiguration.NProc()
	maxHeight := make([]int, n)
	for pid := 0; pid < n; pid++ {
		maxHeight[pid] = -1
	}
	newPoset := &poset{
		nProcesses:    n,
		primeUnits:    []gomel.SlottedUnits{},
		unitsByHeight: []gomel.SlottedUnits{},
		maximalHeight: maxHeight,
		unitByHash:    make(map[gomel.Hash]gomel.Unit),
	}
	return newPoset
}

func (p *poset) AddUnit(pu gomel.Preunit, callback func(gomel.Preunit, gomel.Unit, error)) {
	p.Lock()
	defer p.Unlock()

	var u unit
	// Dehashing parents
	u.parents = []gomel.Unit{}
	for _, parentHash := range pu.Parents() {
		if _, ok := p.unitByHash[parentHash]; !ok {
			callback(pu, nil, gomel.NewDataError("unit with provided hash doesn't exist in our poset"))
			return
		}
		u.parents = append(u.parents, p.unitByHash[parentHash])
	}
	// Setting height, creator, veresion, hash
	u.creator = pu.Creator()
	if len(u.parents) == 0 {
		u.height = 0
	} else {
		u.height = u.parents[0].Height() + 1
	}
	u.hash = *pu.Hash()
	if len(p.unitsByHeight) <= u.height {
		u.version = 0
	} else {
		u.version = len(p.unitsByHeight[u.height].Get(u.creator))
	}
	// Setting level of u
	setLevel(&u, p)

	//Setting poset variables
	if u.Height() == 0 {
		if len(p.unitsByHeight) == 0 {
			p.unitsByHeight = append(p.unitsByHeight, newSlottedUnits(p.nProcesses))
		}
		p.unitsByHeight[0].Set(u.Creator(), append(p.unitsByHeight[0].Get(u.Creator()), &u))
		if len(p.primeUnits) == 0 {
			p.primeUnits = append(p.primeUnits, newSlottedUnits(p.nProcesses))
		}
		p.primeUnits[0].Set(u.Creator(), append(p.primeUnits[0].Get(u.Creator()), &u))
	} else {
		if gomel.Prime(&u) {
			if len(p.primeUnits) <= u.Level() {
				p.primeUnits = append(p.primeUnits, newSlottedUnits(p.nProcesses))
			}
			p.primeUnits[u.Level()].Set(u.Creator(), append(p.primeUnits[u.Level()].Get(u.Creator()), &u))
		}
		if len(p.unitsByHeight) <= u.Height() {
			p.unitsByHeight = append(p.unitsByHeight, newSlottedUnits(p.nProcesses))
		}
		p.unitsByHeight[u.Height()].Set(u.Creator(), append(p.unitsByHeight[u.Height()].Get(u.Creator()), &u))
	}
	if u.Height() > p.maximalHeight[u.Creator()] {
		p.maximalHeight[u.Creator()] = u.Height()
	}
	p.unitByHash[*u.Hash()] = &u

	callback(pu, &u, nil)
}

func (p *poset) PrimeUnits(level int) gomel.SlottedUnits {
	p.RLock()
	defer p.RUnlock()
	return p.primeUnits[level]
}

func (p *poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
	p.RLock()
	defer p.RUnlock()
	su := newSlottedUnits(p.nProcesses)
	for pid := 0; pid < p.nProcesses; pid++ {
		if p.maximalHeight[pid] >= 0 {
			su.Set(pid, p.unitsByHeight[p.maximalHeight[pid]].Get(pid))
		}
	}
	return su
}

func (p *poset) NProc() int {
	// nProcesses doesn't change so no lock needed
	return p.nProcesses
}

func (p *poset) IsQuorum(number int) bool {
	// nProcesses doesn't change so no lock needed
	return 3*number > 2*p.nProcesses
}

func setLevel(u *unit, p *poset) {
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
	queue := []gomel.Unit{u}
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
	if p.IsQuorum(len(seenProcesses)) {
		u.level = maxLevelBelow + 1
	}
}
