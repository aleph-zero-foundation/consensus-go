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
	var w gomel.Unit
	for w = u; w != nil && w.Height() > v.Height(); w = gomel.Predecessor(w) {
	}
	if w == nil {
		return false
	}
	return *w.Hash() == *v.Hash()
}

func (u *freeUnit) Above(v gomel.Unit) bool {
	if v == nil || u == nil {
		return false
	}
	for _, w := range u.Floor()[v.Creator()] {
		// This check is probably redundant, but for now let's keep it just in case.
		if w.Creator() != v.Creator() {
			panic("AboveWithinProc: Different creators")
		}
		if w.AboveWithinProc(v) {
			return true
		}
	}
	return false
}

func (u *unitInDag) AboveWithinProc(v gomel.Unit) bool {
	if u.Height() < v.Height() {
		return false
	}
	if vInDag, ok := v.(*unitInDag); ok && v.Height() <= commonForkingHeight(u, vInDag) {
		return true
	}
	// Either we have a fork or a different type of unit, either way no optimization is possible.
	return u.Unit.AboveWithinProc(v)
}
