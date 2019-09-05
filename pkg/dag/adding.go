package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func (dag *dag) AddUnit(pu gomel.Preunit, callback gomel.Callback) {
	gomel.AddUnit(dag, pu, callback)
}

func (dag *dag) Decode(pu gomel.Preunit) (gomel.Unit, error) {
	if pu.Creator() < 0 || pu.Creator() >= dag.nProcesses {
		return nil, gomel.NewDataError("invalid creator")
	}
	if u := dag.Get([]*gomel.Hash{pu.Hash()}); u[0] != nil {
		return nil, gomel.NewDuplicateUnit(u[0])
	}
	possibleParents, unknown := dag.units.get(pu.Parents())
	if unknown > 0 {
		return nil, gomel.NewUnknownParents(unknown)
	}
	return newUnit(pu, possibleParents, dag.nProcesses), nil
}

func (dag *dag) Check(gomel.Unit) error {
	return nil
}

func (dag *dag) Emplace(u gomel.Unit) gomel.Unit {
	result := emplaced(u, dag)
	if gomel.Prime(result) {
		dag.addPrime(result)
	}
	dag.units.add(result)
	dag.updateMaximal(result)
	return result
}

func (dag *dag) addPrime(u gomel.Unit) {
	if u.Level() >= dag.primeUnits.Len() {
		dag.primeUnits.extendBy(10)
	}
	su, _ := dag.primeUnits.getLevel(u.Level())
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
		if !v.Below(u) {
			newMaxByCreator = append(newMaxByCreator, v)
		}
	}
	newMaxByCreator = append(newMaxByCreator, u)
	dag.maxUnits.Set(creator, newMaxByCreator)
}
