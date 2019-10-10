package alerter

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type alertDag struct {
	gomel.Dag
	alert *Alerter
}

func (dag *alertDag) Check(u gomel.Unit) error {
	if err := dag.Dag.Check(u); err != nil {
		return err
	}
	dag.alert.Lock(u.Creator())
	defer dag.alert.Unlock(u.Creator())
	if dag.isForkerUnit(u) {
		if !dag.alert.CommitmentTo(u) {
			return gomel.NewMissingDataError("commitment to fork")
		}
	}
	return nil
}

func (dag *alertDag) Emplace(u gomel.Unit) (gomel.Unit, error) {
	dag.alert.Lock(u.Creator())
	defer dag.alert.Unlock(u.Creator())
	if dag.alert.IsForker(u.Creator()) {
		if !dag.alert.CommitmentTo(u) {
			return nil, gomel.NewMissingDataError("commitment to fork")
		}
	}
	return dag.Dag.Emplace(u)
}

func (dag *alertDag) isForkerUnit(u gomel.Unit) bool {
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
func Wrap(dag gomel.Dag, alerter *Alerter) gomel.Dag {
	return &alertDag{dag, alerter}
}
