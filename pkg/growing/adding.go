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
func (dag *Dag) AddUnit(pu gomel.Preunit, rs gomel.RandomSource, callback func(gomel.Preunit, gomel.Unit, error)) {
	if pu.Creator() < 0 || pu.Creator() >= dag.nProcesses {
		callback(pu, nil, gomel.NewDataError("Invalid creator."))
		return
	}
	toAdd := &unitBuilt{
		preunit: pu,
		result:  newUnit(pu),
		rs:      rs,
		done:    callback,
	}
	dag.adders[pu.Creator()] <- toAdd
}

func (dag *Dag) verifySignature(pu gomel.Preunit) error {
	if !dag.pubKeys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("Invalid Signature")
	}
	return nil
}

func (dag *Dag) addPrime(u gomel.Unit) {
	if u.Level() >= dag.primeUnits.Len() {
		dag.primeUnits.extendBy(10)
	}
	su, _ := dag.primeUnits.getLevel(u.Level())
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

func (dag *Dag) updateMaximal(u gomel.Unit) {
	creator := u.Creator()
	maxByCreator := dag.maxUnits.Get(creator)
	newMaxByCreator := make([]gomel.Unit, 0)
	// The below code works properly assuming that no unit in the Dag created by creator is >= u
	for _, v := range maxByCreator {
		if !v.Below(u) {
			newMaxByCreator = append(newMaxByCreator, v)
		}
	}
	newMaxByCreator = append(newMaxByCreator, u)
	dag.maxUnits.Set(creator, newMaxByCreator)
}

func (dag *Dag) dehashParents(ub *unitBuilt) error {
	if u := dag.Get([]*gomel.Hash{ub.preunit.Hash()}); u[0] != nil {
		return gomel.NewDuplicateUnit(u[0])
	}
	possibleParents := dag.units.get(ub.preunit.Parents())
	for _, parent := range possibleParents {
		if parent == nil {
			return gomel.NewUnknownParent()
		}
		ub.result.addParent(parent)
	}
	return nil
}

func (dag *Dag) prepareUnit(ub *unitBuilt) error {
	err := dag.dehashParents(ub)
	if err != nil {
		return err
	}
	err = dag.checkBasicParentsCorrectness(ub.result)
	if err != nil {
		return err
	}
	ub.result.initialize(dag)
	return dag.checkCompliance(ub.result, ub.rs)
}

func (dag *Dag) addUnit(ub *unitBuilt) {
	err := dag.prepareUnit(ub)
	if err != nil {
		ub.done(ub.preunit, nil, err)
		return
	}
	ub.rs.Update(ub.result)
	if gomel.Prime(ub.result) {
		dag.addPrime(ub.result)
	}
	dag.units.add(ub.result)
	dag.updateMaximal(ub.result)
	ub.done(ub.preunit, ub.result, nil)
}

func (dag *Dag) adder(incoming chan *unitBuilt) {
	defer dag.tasks.Done()
	for ub := range incoming {
		dag.addUnit(ub)
	}
}
