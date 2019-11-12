package forking

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func (a *AlertHandler) ambiguousParentsHandler(pu gomel.Preunit, err error, source uint16) ([]gomel.Unit, error) {
	switch e := err.(type) {
	case *gomel.AmbiguousParents:
		parents := make([]gomel.Unit, 0, len(e.Units))
		for _, us := range e.Units {
			p, err := a.Disambiguate(us, pu)
			if err != nil {
				switch err.(type) {
				case *noCommitment:
					err2 := a.RequestCommitment(pu, source)
					if err2 != nil {
						return nil, err2
					}
				default:
					return nil, err
				}
			}
			parents = append(parents, p)
		}
		if *gomel.CombineHashes(gomel.ToHashes(parents)) != pu.View().ControlHash {
			return nil, gomel.NewDataError("wrong control hash")
		}
		return parents, nil
	default:
		return nil, err
	}
}

func (a *AlertHandler) checkErrorHandler(u gomel.Unit, err error, source uint16) error {
	switch err.(type) {
	case *noCommitment:
		return a.RequestCommitment(u, source)
	default:
		return err
	}
}

func (a *AlertHandler) checkCommitment(u gomel.Unit) error {
	a.Lock(u.Creator())
	if a.handleForkerUnit(u) && !a.HasCommitmentTo(u) {
		a.Unlock(u.Creator())
		return missingCommitment("missing commitment to fork")
	}
	return nil
}

func (a *AlertHandler) handleForkerUnit(u gomel.Unit) bool {
	creator := u.Creator()
	if a.IsForker(creator) {
		return true
	}
	maxes := a.dag.MaximalUnitsPerProcess().Get(creator)
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
		a.Raise(proof)
		return true
	}
	return false
}
