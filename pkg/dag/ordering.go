package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func commonForkingHeight(u, v *unit) int {
	if u.forkingHeight < v.forkingHeight {
		return u.forkingHeight
	}
	return v.forkingHeight
}

func brutalAboveWithinProc(u, v gomel.Unit) bool {
	for u != nil && u.Height() > v.Height() {
		u = gomel.Predecessor(u)
	}
	if u == nil {
		return false
	}
	return *u.Hash() == *v.Hash()
}

func aboveWithinProc(u, v gomel.Unit) (bool, error) {
	if u.Creator() != v.Creator() {
		return false, gomel.NewDataError("Different creators")
	}
	if u.Height() < v.Height() {
		return false, nil
	}

	uWithForkingHeight, uKnowsForkingHeight := u.(*unit)
	vWithForkingHeight, vKnowsForkingHeight := v.(*unit)

	if uKnowsForkingHeight && vKnowsForkingHeight && vWithForkingHeight.height <= commonForkingHeight(uWithForkingHeight, vWithForkingHeight) {
		return true, nil
	}

	// Either we have a fork or a different type of unit, either way no optimization is possible.
	return brutalAboveWithinProc(u, v), nil
}

func (u *unit) Above(v gomel.Unit) bool {
	if v == nil || u == nil {
		return false
	}
	for _, w := range u.floor[v.Creator()] {

		if ok, _ := aboveWithinProc(w, v); ok {
			return true
		}
	}
	return false
}

func (u *freeUnit) Above(v gomel.Unit) bool {
	if v == nil || u == nil {
		return false
	}
	for _, w := range u.Floor()[v.Creator()] {
		if brutalAboveWithinProc(w, v) {
			return true
		}
	}
	return false
}
