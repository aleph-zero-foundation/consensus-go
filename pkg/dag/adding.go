package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/unit"
)

func (dag *dag) DecodeParents(pu gomel.Preunit) ([]gomel.Unit, error) {
	if u := dag.GetUnit(pu.Hash()); u != nil {
		return nil, gomel.NewDuplicateUnit(u)
	}
	heights := pu.View().Heights
	possibleParents, unknown := dag.heightUnits.get(heights)
	if unknown > 0 {
		return nil, gomel.NewUnknownParents(unknown)
	}
	parents := make([]gomel.Unit, dag.nProcesses)
	for i, units := range possibleParents {
		if heights[i] == -1 {
			continue
		}
		if len(units) > 1 {
			return nil, gomel.NewAmbiguousParents(possibleParents)
		}
		parents[i] = units[0]
	}
	if *gomel.CombineHashes(gomel.ToHashes(parents)) != pu.View().ControlHash {
		return nil, gomel.NewDataError("wrong control hash")
	}
	return parents, nil
}

func (dag *dag) BuildUnit(pu gomel.Preunit, parents []gomel.Unit) gomel.Unit {
	return unit.FromPreunit(pu, parents)
}

func (dag *dag) Check(u gomel.Unit) error {
	for _, check := range dag.checks {
		if err := check(u, dag); err != nil {
			return err
		}
	}
	return nil
}

func (dag *dag) Insert(u gomel.Unit) {
	u = unit.Embed(u, dag)
	for _, hook := range dag.preInsert {
		hook(u)
	}
	dag.updateUnitsOnHeight(u)
	dag.addPrime(u)
	dag.units.add(u)
	dag.updateMaximal(u)
	for _, hook := range dag.postInsert {
		hook(u)
	}
}

func (dag *dag) addPrime(u gomel.Unit) {
	if u.Level() >= dag.primeUnits.Len() {
		dag.primeUnits.extendBy(10)
	}
	su, _ := dag.primeUnits.getFiber(u.Level())
	creator := u.Creator()
	oldPrimes := su.Get(creator)
	primesByCreator := make([]gomel.Unit, len(oldPrimes), len(oldPrimes)+1)
	copy(primesByCreator, oldPrimes)
	primesByCreator = append(primesByCreator, u)
	su.Set(creator, primesByCreator)
}

func (dag *dag) updateMaximal(u gomel.Unit) {
	creator := u.Creator()
	maxByCreator := dag.maxUnits.Get(creator)
	newMaxByCreator := make([]gomel.Unit, 0)
	// The below code works properly assuming that no unit in the dag created by creator is >= u
	for _, v := range maxByCreator {
		if !gomel.Above(u, v) {
			newMaxByCreator = append(newMaxByCreator, v)
		}
	}
	newMaxByCreator = append(newMaxByCreator, u)
	dag.maxUnits.Set(creator, newMaxByCreator)
}

func (dag *dag) updateUnitsOnHeight(u gomel.Unit) {
	height := u.Height()
	creator := u.Creator()
	if height >= dag.heightUnits.Len() {
		dag.heightUnits.extendBy(10)
	}
	su, _ := dag.heightUnits.getFiber(height)

	oldUnitsOnHeightByCreator := su.Get(creator)
	unitsOnHeightByCreator := make([]gomel.Unit, len(oldUnitsOnHeightByCreator), len(oldUnitsOnHeightByCreator)+1)
	copy(unitsOnHeightByCreator, oldUnitsOnHeightByCreator)
	unitsOnHeightByCreator = append(unitsOnHeightByCreator, u)
	su.Set(creator, unitsOnHeightByCreator)
}
