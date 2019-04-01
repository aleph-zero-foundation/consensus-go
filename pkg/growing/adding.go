package growing

import (
	a "gitlab.com/alephledger/consensus-go/pkg"
)

type unitBuilt struct {
	preunit a.Preunit
	result  *unit
	done    func(a.Preunit, a.Unit, error)
}

// Adds the provided Preunit to the poset as a Unit.
// When done calls the callback.
func (p *Poset) AddUnit(pu a.Preunit, callback func(a.Preunit, a.Unit, error)) {
	if pu.Creator() < 0 || pu.Creator() > p.nProcesses {
		callback(pu, nil, a.NewDataError("Invalid creator."))
		return
	}
	toAdd := &unitBuilt{
		preunit: pu,
		result:  newUnit(pu),
		done:    callback,
	}
	p.adders[pu.Creator()] <- toAdd
}

func (p *Poset) checkSigature(pu a.Preunit) error {
	// TODO: actually check
	return nil
}

func setHeight(ub *unitBuilt) error {
	if len(ub.result.parents) == 0 {
		ub.result.setHeight(0)
		return nil
	}
	if ub.result.Parents()[0].Creator() != ub.preunit.Creator() {
		return a.NewComplianceError("Not descendant of first parent")
	}
	ub.result.setHeight(ub.result.Parents()[0].Height() + 1)
	return nil
}

func (p *Poset) computeLevel(ub *unitBuilt) {
	// TODO: actually compute
	ub.result.setLevel(0)
}

func (p *Poset) checkCompliance(u a.Unit) error {
	// TODO: actually check, also should be separate file, cause it'll be long
	return nil
}

func (p *Poset) addPrime(u a.Unit) {
	// TODO: actually add
}

func (p *Poset) updateMaximal(u a.Unit) {
	// TODO: actually update
}

func (p *Poset) adder(incoming chan *unitBuilt) {
	for {
		ub := <-incoming
		if ub == nil {
			// TODO: some cleanup here?
			return
		}
		err := p.units.dehashParents(ub)
		if err != nil {
			ub.done(ub.preunit, nil, err)
			continue
		}
		err = p.checkSigature(ub.preunit)
		if err != nil {
			ub.done(ub.preunit, nil, err)
			continue
		}
		err = setHeight(ub)
		if err != nil {
			ub.done(ub.preunit, nil, err)
			continue
		}
		ub.result.computeFloor()
		p.computeLevel(ub)
		err = p.checkCompliance(ub.result)
		if err != nil {
			ub.done(ub.preunit, nil, err)
			continue
		}
		if a.Prime(ub.result) {
			p.addPrime(ub.result)
		}
		p.units.add(ub.result)
		ub.done(ub.preunit, ub.result, nil)
		p.updateMaximal(ub.result)
	}
}
