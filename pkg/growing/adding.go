package growing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"math"
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

func (p *Poset) checkSignature(pu gomel.Preunit) error {
	// TODO: actually check
	return nil
}

func (p *Poset) precheck(ub *unitBuilt) error {
	err := p.checkSignature(ub.preunit)
	if err != nil {
		return err
	}
	if len(ub.result.Parents()) == 0 {
		return nil
	}
	if len(ub.result.Parents()) < 2 {
		return gomel.NewDataError("Not enough parents")
	}
	if ub.result.Parents()[0].Creator() != ub.preunit.Creator() {
		return gomel.NewComplianceError("Not descendant of first parent")
	}
	return nil
}

func (p *Poset) computeLevel(ub *unitBuilt) {
	if len(ub.result.parents) == 0 {
		ub.result.setLevel(0)
	} else {
		maxLevelParents := 0
		for _, w := range ub.result.parents {
			if w.Level() > maxLevelParents {
				maxLevelParents = w.Level()
			}
		}
		nSeen := 0
		for pid := 0; pid < p.nProcesses; pid++ {
			pidSeen := 0
			for _, v := range p.PrimeUnits(maxLevelParents).Get(pid) {
				if p.Below(v, ub.result) {
					pidSeen = 1
					break
				}
			}
			nSeen += pidSeen
			// optimization to not loop over all processes if quorum cannot be reached anyway
			if !p.isQuorum(nSeen + p.nProcesses - 1 - pid) {
                break
			}
		}
		if p.isQuorum(nSeen) {
			ub.result.setLevel(maxLevelParents + 1)
		} else {
			ub.result.setLevel(maxLevelParents)
		}

	}

}

func (p *Poset) checkCompliance(u gomel.Unit) error {
	// TODO: actually check, also should be separate file, cause it'll be long
	return nil
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
		// It is assumed that p.Below implements strict inequality <
		if !p.Below(v, u) {
			newMaxByCreator = append(newMaxByCreator, v)
		}
	}
	newMaxByCreator = append(newMaxByCreator, u)
	// Only the adder goroutine corresponding to this creator is ever writing to p.maxUnits[creator].
	// Hence p.maxUnits[creator] cannot change between the Get() above and the Set() below.
	p.maxUnits.Set(creator, newMaxByCreator)
}

func (p *Poset) computeForkingHeight(u *unit) {
	// this implementation works as long as there is no race for writing/reading to p.maxUnits, i.e.
	// as long as units created by one process are added atomically
	if len(u.parents) == 0 {
		if len(p.MaximalUnitsPerProcess().Get(u.creator)) > 0 {
			//this is a forking dealing unit
			u.forkingHeight = 0
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
			for _, v := range p.MaximalUnitsPerProcess().Get(u.creator) {
				for w := v.(*unit); w != up; w = w.Parents()[0].(*unit) {
					w.forkingHeight = up.height
				}
			}
		}
	}
}

func (p *Poset) prepareUnit(ub *unitBuilt) error {
	err := p.units.dehashParents(ub)
	if err != nil {
		return err
	}
	err = p.precheck(ub)
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
