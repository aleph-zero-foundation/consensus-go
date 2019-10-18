package dag

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func commonForkingHeight(u, v *unitInDag) int {
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

func aboveWithinProc(u, v gomel.Unit) bool {
	if u.Creator() != v.Creator() {
		panic("aboveWithinProc: Different creators")
	}
	if u.Height() < v.Height() {
		return false
	}

	uWithForkingHeight, uKnowsForkingHeight := u.(*unitInDag)
	vWithForkingHeight, vKnowsForkingHeight := v.(*unitInDag)

	if uKnowsForkingHeight && vKnowsForkingHeight && v.Height() <= commonForkingHeight(uWithForkingHeight, vWithForkingHeight) {
		return true
	}

	// Either we have a fork or a different type of unit, either way no optimization is possible.
	return brutalAboveWithinProc(u, v)
}

func (u *unitInDag) Above(v gomel.Unit) bool {
	if v == nil || u == nil {
		return false
	}
	for _, w := range u.Floor()[v.Creator()] {
		if aboveWithinProc(w, v) {
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
