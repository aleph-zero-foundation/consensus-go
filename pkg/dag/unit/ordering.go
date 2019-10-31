package unit

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func commonForkingHeight(u, v *unitInDag) int {
	if u.forkingHeight < v.forkingHeight {
		return u.forkingHeight
	}
	return v.forkingHeight
}

func (u *freeUnit) AboveWithinProc(v gomel.Unit) bool {
	if u.Creator() != v.Creator() {
		return false
	}
	var w gomel.Unit
	for w = u; w != nil && w.Height() > v.Height(); w = gomel.Predecessor(w) {
	}
	if w == nil {
		return false
	}
	return *w.Hash() == *v.Hash()
}

func (u *unitInDag) AboveWithinProc(v gomel.Unit) bool {
	if u.Height() < v.Height() || u.Creator() != v.Creator() {
		return false
	}
	if vInDag, ok := v.(*unitInDag); ok && v.Height() <= commonForkingHeight(u, vInDag) {
		return true
	}
	// Either we have a fork or a different type of unit, either way no optimization is possible.
	return u.Unit.AboveWithinProc(v)
}
