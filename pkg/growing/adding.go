package growing

import (
	"math"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type unitBuilt struct {
	preunit gomel.Preunit
	result  *unit
	done    func(gomel.Preunit, gomel.Unit, error)
}

// Adds the provided Preunit to the poset as a Unit.
// When done calls the callback.
func (p *Poset) AddUnit(pu gomel.Preunit, callback func(gomel.Preunit, gomel.Unit, error)) {
	if pu.Creator() < 0 || pu.Creator() >= p.nProcesses {
		callback(pu, nil, gomel.NewDataError("Invalid creator."))
		return
	}
	toAdd := &unitBuilt{
		preunit: pu,
		result:  newUnit(pu),
		done:    callback,
	}
	p.adders[pu.Creator()] <- toAdd
}

func (p *Poset) verifySignature(pu gomel.Preunit) error {
	if !p.pubKeys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("Invalid Signature")
	}
	return nil
}

func computeLevelUsingFloor(p *Poset, unit *unit) int {
	// NOTE: unit.floor[pid][0] should be occupied by a unit with maximal level
	maxLevelParents := 0
	for _, w := range unit.parents {
		wLevel := w.Level()
		if wLevel > maxLevelParents {
			maxLevelParents = wLevel
		}
	}

	nSeen := 0
	for pid, vs := range unit.floor {

		for _, unit := range vs {
			if unit.Level() == maxLevelParents {
				nSeen++
				break
			}
		}

		// optimization to not loop over all processes if quorum cannot be reached anyway
		if !p.IsQuorum(nSeen + (p.nProcesses - (pid + 1))) {
			break
		}

		if p.IsQuorum(nSeen) {
			return maxLevelParents + 1
		}
	}
	return maxLevelParents
}

func (p *Poset) computeLevel(ub *unitBuilt) {
	if gomel.Dealing(ub.result) {
		ub.result.setLevel(0)
		return
	}
	level := computeLevelUsingFloor(p, ub.result)
	ub.result.setLevel(level)
}

func (p *Poset) addPrime(u gomel.Unit) {
	if u.Level() > p.primeUnits.getHeight() {
		p.primeUnits.extendBy(10)
	}
	su, _ := p.primeUnits.getLevel(u.Level())
	creator := u.Creator()
	primesByCreator := append(su.Get(creator), u)
	// this assumes that we are adding u for the first time
	su.Set(creator, primesByCreator)
}

func (p *Poset) updateMaximal(u gomel.Unit) {
	creator := u.Creator()
	maxByCreator := p.maxUnits.Get(creator)
	newMaxByCreator := make([]gomel.Unit, 0)
	// The below code works properly assuming that no unit in the Poset created by creator is >= u
	for _, v := range maxByCreator {
		if !v.Below(u) {
			newMaxByCreator = append(newMaxByCreator, v)
		}
	}
	newMaxByCreator = append(newMaxByCreator, u)
	p.maxUnits.Set(creator, newMaxByCreator)
}

func (p *Poset) computeForkingHeight(u *unit) {
	// this implementation works as long as there is no race for writing/reading to p.maxUnits, i.e.
	// as long as units created by one process are added atomically
	if len(u.parents) == 0 {
		if len(p.MaximalUnitsPerProcess().Get(u.creator)) > 0 {
			//this is a forking dealing unit
			u.forkingHeight = -1
		} else {
			u.forkingHeight = math.MaxInt32
		}
		return
	}
	up := u.parents[0].(*unit)
	found := false
	for _, v := range p.MaximalUnitsPerProcess().Get(u.creator) {
		if v == up {
			found = true
			break
		}
	}
	if found {
		u.forkingHeight = up.forkingHeight
	} else {
		// there is already a unit that has up as a predecessor, hence u is a fork
		if up.forkingHeight < up.height {
			u.forkingHeight = up.forkingHeight
		} else {
			u.forkingHeight = up.height
		}
	}
}

func (p *Poset) prepareUnit(ub *unitBuilt) error {
	err := p.units.dehashParents(ub)
	if err != nil {
		return err
	}
	err = p.checkBasicParentsCorrectness(ub.result)
	if err != nil {
		return err
	}
	ub.result.computeHeight()
	ub.result.computeFloor(p.nProcesses)
	p.computeLevel(ub)
	p.computeForkingHeight(ub.result)
	return p.checkCompliance(ub.result)
}

func (p *Poset) addUnit(ub *unitBuilt) {
	err := p.prepareUnit(ub)
	if err != nil {
		ub.done(ub.preunit, nil, err)
		return
	}
	if gomel.Prime(ub.result) {
		p.addPrime(ub.result)
	}
	p.units.add(ub.result)
	ub.done(ub.preunit, ub.result, nil)
	p.updateMaximal(ub.result)
}

func (p *Poset) adder(incoming chan *unitBuilt) {
	defer p.tasks.Done()
	for ub := range incoming {
		p.addUnit(ub)
	}
}
