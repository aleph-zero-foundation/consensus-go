package growing

import (
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

func setHeight(ub *unitBuilt) {
	if len(ub.result.parents) == 0 {
		ub.result.setHeight(0)
	} else {
		ub.result.setHeight(ub.result.Parents()[0].Height() + 1)
	}
}

func (p *Poset) computeLevel(ub *unitBuilt) {
	// TODO: actually compute
	ub.result.setLevel(0)
}

func (p *Poset) checkCompliance(u gomel.Unit) error {
	// TODO: actually check, also should be separate file, cause it'll be long
	return nil
}

func (p *Poset) addPrime(u gomel.Unit) {
	// TODO: actually add
}

func (p *Poset) updateMaximal(u gomel.Unit) {
	// TODO: actually update
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
	setHeight(ub)
	ub.result.computeFloor()
	p.computeLevel(ub)
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
