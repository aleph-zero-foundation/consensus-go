package forking

import (
	//"gitlab.com/alephledger/consensus-go/pkg/dag/unit"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type alertDag struct {
	gomel.Dag
	alert *AlertHandler
}

/*
func (dag *alertDag) Decode(pu gomel.Preunit) (gomel.Unit, error) {
	u, err := dag.Dag.Decode(pu)
	if err == nil {
		return u, nil
	}
	switch e := err.(type) {
	case *gomel.AmbiguousParents:
		parents := make([]gomel.Unit, 0, len(e.Units))
		for _, us := range e.Units {
			p, err := dag.alert.Disambiguate(us, pu)
			if err != nil {
				return nil, err
			}
			parents = append(parents, p)
		}
		if *gomel.CombineHashes(gomel.ToHashes(parents)) != pu.View().ControlHash {
			return nil, gomel.NewDataError("wrong control hash")
		}
		return unit.New(pu, parents), nil
	default:
		return u, err
	}
}*/

func (dag *alertDag) Prepare(u gomel.Unit) (gomel.Unit, error) {
	u, err := dag.Dag.Prepare(u)
	if err != nil {
		return nil, err
	}
	dag.alert.Lock(u.Creator())
	if dag.handleForkerUnit(u) {
		if !dag.alert.HasCommitmentTo(u) {
			dag.alert.Unlock(u.Creator())
			return nil, missingCommitmentToForkError
		}
	}
	return u, nil
}

func (dag *alertDag) Insert(u gomel.Unit) {
	dag.Dag.Insert(u)
	dag.alert.Unlock(u.Creator())
}

func (dag *alertDag) handleForkerUnit(u gomel.Unit) bool {
	creator := u.Creator()
	if dag.alert.IsForker(creator) {
		return true
	}
	maxes := dag.Dag.MaximalUnitsPerProcess().Get(creator)
	if len(maxes) == 0 {
		return false
	}
	// There can be only one, because the creator is not yet a forker.
	max := maxes[0]
	if max.Height() >= u.Height() {
		proof := newForkingProof(u, max)
		if proof == nil {
			return false
		}
		dag.alert.Raise(proof)
		return true
	}
	return false
}

// Wrap the dag to support alerts when forks are encountered. The returned service handles raising and accepting alerts.
func Wrap(dag gomel.Dag, alerter *AlertHandler) gomel.Dag {
	return &alertDag{dag, alerter}
}
