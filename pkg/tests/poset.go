package tests

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

// Poset is a basic implementation of poset for testing
type Poset struct {
	sync.RWMutex
	nProcesses int
	primeUnits []gomel.SlottedUnits
	// maximalHeight is the maximalHeight of a unit created per process
	maximalHeight []int
	unitsByHeight []gomel.SlottedUnits
	unitByHash    map[gomel.Hash]gomel.Unit
	tcByHash      map[gomel.Hash]*tcoin.ThresholdCoin
}

func newPoset(posetConfiguration gomel.PosetConfig) *Poset {
	n := posetConfiguration.NProc()
	maxHeight := make([]int, n)
	for pid := 0; pid < n; pid++ {
		maxHeight[pid] = -1
	}
	newPoset := &Poset{
		nProcesses:    n,
		primeUnits:    []gomel.SlottedUnits{},
		unitsByHeight: []gomel.SlottedUnits{},
		maximalHeight: maxHeight,
		unitByHash:    make(map[gomel.Hash]gomel.Unit),
		tcByHash:      make(map[gomel.Hash]*tcoin.ThresholdCoin),
	}
	return newPoset
}

// AddThresholdCoin adds threshold coin to the poset
func (p *Poset) AddThresholdCoin(h *gomel.Hash, tc *tcoin.ThresholdCoin) {
	p.Lock()
	defer p.Unlock()
	p.tcByHash[*h] = tc
}

// RemoveThresholdCoin removes threshold coin from the poset
func (p *Poset) RemoveThresholdCoin(h *gomel.Hash) {
	p.Lock()
	defer p.Unlock()
	delete(p.tcByHash, *h)
}

// ThresholdCoin returns local threshold coin dealt by dealing unit having given hash
// nil for hashes of non-dealing units
func (p *Poset) ThresholdCoin(h *gomel.Hash) *tcoin.ThresholdCoin {
	p.Lock()
	defer p.Unlock()
	if tc, ok := p.tcByHash[*h]; ok {
		return tc
	}
	return nil
}

// GetCRP is a dummy implementation of a common random permutation
func (p *Poset) GetCRP(level int) []int {
	permutation := make([]int, p.NProc())
	for i := 0; i < p.NProc(); i++ {
		permutation[i] = (i + level) % p.NProc()
	}
	return permutation
}

// AddUnit adds a unit in a thread safe manner without trying to be clever.
func (p *Poset) AddUnit(pu gomel.Preunit, callback func(gomel.Preunit, gomel.Unit, error)) {
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
	// Setting height, creator, signature, version, hash
	u.creator = pu.Creator()
	if len(u.parents) == 0 {
		u.height = 0
	} else {
		u.height = u.parents[0].Height() + 1
	}
	u.signature = pu.Signature()
	u.hash = *pu.Hash()
	u.tcData = pu.ThresholdCoinData()
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

// PrimeUnits returns the prime units at the given level.
func (p *Poset) PrimeUnits(level int) gomel.SlottedUnits {
	p.RLock()
	defer p.RUnlock()
	return p.primeUnits[level]
}

// MaximalUnitsPerProcess returns the maximal units for all processes.
func (p *Poset) MaximalUnitsPerProcess() gomel.SlottedUnits {
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

// Get retunrs the units with the given hashes or nil, when it doesn't find them.
func (p *Poset) Get(hashes []gomel.Hash) []gomel.Unit {
	p.RLock()
	defer p.RUnlock()
	result := make([]gomel.Unit, len(hashes))
	for i, h := range hashes {
		result[i] = p.unitByHash[h]
	}
	return result
}

// NProc returns the number of processes in this poset.
func (p *Poset) NProc() int {
	// nProcesses doesn't change so no lock needed
	return p.nProcesses
}

// IsQuorum checks whether the provided number of processes constitutes a quorum.
func (p *Poset) IsQuorum(number int) bool {
	// nProcesses doesn't change so no lock needed
	return 3*number > 2*p.nProcesses
}

func setLevel(u *unit, p *Poset) {
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

func (p *Poset) getPrimeUnitsOnLevel(level int) []gomel.Unit {
	result := []gomel.Unit{}
	for pid := 0; pid < p.NProc(); pid++ {
		result = append(result, p.primeUnits[level].Get(pid)...)
	}
	return result
}
