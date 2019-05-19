package growing

import (
	"fmt"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type unitBuilt struct {
	preunit gomel.Preunit
	result  *unit
	done    func(gomel.Preunit, gomel.Unit, error)
}

// AddUnit adds the provided Preunit to the poset as a Unit.
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

func (p *Poset) addPrime(u gomel.Unit) {
	if u.Level() >= p.primeUnits.Len() {
		p.primeUnits.extendBy(10)
	}
	su, err := p.primeUnits.getLevel(u.Level())
	if err != nil {
		fmt.Println(err)
	}
	if su == nil {
		fmt.Println("nil su", u.Level(), p.primeUnits.Len())
	}
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

func (p *Poset) dehashParents(ub *unitBuilt) error {
	if u := p.units.get([]gomel.Hash{*ub.preunit.Hash()}); u[0] != nil {
		return &gomel.DuplicateUnit{}
	}
	possibleParents := p.units.get(ub.preunit.Parents())
	for _, parent := range possibleParents {
		if parent == nil {
			return gomel.NewDataError("Missing parent")
		}
		ub.result.addParent(parent)
	}
	return nil
}

func (p *Poset) prepareUnit(ub *unitBuilt) error {
	err := p.dehashParents(ub)
	if err != nil {
		return err
	}
	err = p.checkBasicParentsCorrectness(ub.result)
	if err != nil {
		return err
	}
	ub.result.initialize(p)
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
