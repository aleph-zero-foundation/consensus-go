package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func (dag *dag) Decode(pu gomel.Preunit) (gomel.Unit, error) {
	if pu.Creator() < 0 || pu.Creator() >= dag.nProcesses {
		return nil, gomel.NewDataError("invalid creator")
	}
	if u := dag.Get([]*gomel.Hash{pu.Hash()}); u[0] != nil {
		return nil, gomel.NewDuplicateUnit(u[0])
	}
	possibleParents := dag.heightUnits.get(pu.View().Heights)
	parents, err := getParents(possibleParents, pu.Creator())
	if err != nil {
		return nil, err
	}

	if unknown := countUnknown(parents, pu.View().Heights); unknown > 0 {
		return nil, gomel.NewUnknownParents(unknown)
	}

	if *gomel.CombineHashes(gomel.ToHashes(parents)) != pu.View().ControlHash {
		return nil, gomel.NewDataError("wrong control hash")
	}

	return NewUnit(pu, parents), nil
}

func countUnknown(parents []gomel.Unit, heights []int) int {
	unknown := 0
	for i, h := range heights {
		if h != -1 && parents[i] == nil {
			unknown++
		}
	}
	return unknown
}

func getParents(units [][]gomel.Unit, pid uint16) ([]gomel.Unit, error) {
	nProc := len(units)
	result := make([]gomel.Unit, nProc)

	for i, us := range units {
		if us != nil {
			result[i] = us[0]
		}
		if len(us) > 1 {
			return nil, gomel.NewAmbiguousParents(units)
		}
	}
	return result, nil
}

func (dag *dag) Check(gomel.Unit) error {
	return nil
}

func (dag *dag) Emplace(u gomel.Unit) (gomel.Unit, error) {
	result := emplaced(u, dag)
	dag.updateUnitsOnHeight(result)
	if gomel.Prime(result) {
		dag.addPrime(result)
	}
	dag.units.add(result)
	dag.updateMaximal(result)
	return result, nil
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
		if !u.Above(v) {
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
