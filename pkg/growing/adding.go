package growing

import (
	"sort"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type unitBuilt struct {
	preunit gomel.Preunit
	result  *unit
	rs      gomel.RandomSource
	done    func(gomel.Preunit, gomel.Unit, error)
}

// AddUnit adds the provided Preunit to the dag as a Unit.
// When done calls the callback.
func (p *Dag) AddUnit(pu gomel.Preunit, rs gomel.RandomSource, callback func(gomel.Preunit, gomel.Unit, error)) {
	if pu.Creator() < 0 || pu.Creator() >= p.nProcesses {
		callback(pu, nil, gomel.NewDataError("Invalid creator."))
		return
	}
	toAdd := &unitBuilt{
		preunit: pu,
		result:  newUnit(pu),
		rs:      rs,
		done:    callback,
	}
	p.adders[pu.Creator()] <- toAdd
}

func (p *Dag) verifySignature(pu gomel.Preunit) error {
	if !p.pubKeys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("Invalid Signature")
	}
	return nil
}

func (p *Dag) addPrime(u gomel.Unit) {
	if u.Level() >= p.primeUnits.Len() {
		p.primeUnits.extendBy(10)
	}
	su, _ := p.primeUnits.getLevel(u.Level())
	creator := u.Creator()
	oldPrimes := su.Get(creator)
	primesByCreator := make([]gomel.Unit, len(oldPrimes), len(oldPrimes)+1)
	copy(primesByCreator, oldPrimes)
	primesByCreator = append(primesByCreator, u)
	// we keep the primes sorted by hash, mostly for ordering
	sort.Slice(primesByCreator, func(i, j int) bool {
		return primesByCreator[i].Hash().LessThan(primesByCreator[j].Hash())
	})
	// this assumes that we are adding u for the first time
	su.Set(creator, primesByCreator)
}

func (p *Dag) updateMaximal(u gomel.Unit) {
	creator := u.Creator()
	maxByCreator := p.maxUnits.Get(creator)
	newMaxByCreator := make([]gomel.Unit, 0)
	// The below code works properly assuming that no unit in the Dag created by creator is >= u
	for _, v := range maxByCreator {
		if !v.Below(u) {
			newMaxByCreator = append(newMaxByCreator, v)
		}
	}
	newMaxByCreator = append(newMaxByCreator, u)
	p.maxUnits.Set(creator, newMaxByCreator)
}

func (p *Dag) dehashParents(ub *unitBuilt) error {
	if u := p.Get([]*gomel.Hash{ub.preunit.Hash()}); u[0] != nil {
		return gomel.NewDuplicateUnit(u[0])
	}
	possibleParents := p.units.get(ub.preunit.Parents())
	for _, parent := range possibleParents {
		if parent == nil {
			return gomel.NewUnknownParent()
		}
		ub.result.addParent(parent)
	}
	return nil
}

func (p *Dag) prepareUnit(ub *unitBuilt) error {
	err := p.dehashParents(ub)
	if err != nil {
		return err
	}
	err = p.checkBasicParentsCorrectness(ub.result)
	if err != nil {
		return err
	}
	ub.result.initialize(p)
	return p.checkCompliance(ub.result, ub.rs)
}

func (p *Dag) addUnit(ub *unitBuilt) {
	err := p.prepareUnit(ub)
	if err != nil {
		ub.done(ub.preunit, nil, err)
		return
	}
	ub.rs.Update(ub.result)
	if gomel.Prime(ub.result) {
		p.addPrime(ub.result)
	}
	p.units.add(ub.result)
	p.updateMaximal(ub.result)
	ub.done(ub.preunit, ub.result, nil)
}

func (p *Dag) adder(incoming chan *unitBuilt) {
	defer p.tasks.Done()
	for ub := range incoming {
		p.addUnit(ub)
	}
}
