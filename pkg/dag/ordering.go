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

func brutalBelowWithinProc(u, v gomel.Unit) bool {
	for v != nil && v.Height() > u.Height() {
		v = gomel.Predecessor(v)
	}
	if v == nil {
		return false
	}
	return v == u
}

func (u *unit) belowWithinProc(v gomel.Unit) (bool, error) {
	if u.Creator() != v.Creator() {
		return false, gomel.NewDataError("Different creators")
	}
	if u.Height() > v.Height() {
		return false, nil
	}

	w, ok := v.(*unit)

	if ok && u.height <= commonForkingHeight(u, w) {
		return true, nil
	}

	// Either we have a fork or a different type of unit, either way no optimization is possible.
	return brutalBelowWithinProc(u, v), nil
}

func (u *unit) Below(v gomel.Unit) bool {
	if v == nil {
		return false
	}
	for _, w := range v.Floor()[u.creator] {

		if ok, _ := u.belowWithinProc(w); ok {
			return true
		}
	}
	return false
}

func (u *freeUnit) Below(v gomel.Unit) bool {
	if v == nil {
		return false
	}
	for _, w := range v.Floor()[u.creator] {

		if ok := brutalBelowWithinProc(u, w); ok {
			return true
		}
	}
	return false
}
