package growing

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

func commonForkingHeight(u, v *unit) int {
	if u.forkingHeight < v.forkingHeight {
		return u.forkingHeight
	}
	return v.forkingHeight
}

func (u *unit) belowWithinProc(v *unit) (bool, error) {
	if u.creator != v.creator {
		return false, gomel.NewDataError("Different creators")
	}
	if u.height > v.height {
		return false, nil
	}

	if u.height <= commonForkingHeight(u, v) {
		return true, nil
	}

	// in forking situation we have to check if going down from v to u.height we find u
	w := v
	for w.height > u.height {
		wVal, err := gomel.Predecessor(w)
		if err != nil {
			return false, err
		}

		w = wVal.(*unit)
	}

	return u == w, nil
}

func (u *unit) aboveWithinProc(v *unit) (bool, error) {
	return v.belowWithinProc(u)
}

// Below checks if a unit u is less than or euqal to a unit v.
func (u *unit) Below(v gomel.Unit) bool {
	var V *unit
	var ok bool
	if V, ok = v.(*unit); !ok {
		// this might be needed in the future when there are special units that separate existing and nonexistent units
	}
	for _, w := range V.floor[u.creator] {

		if ok, _ := u.belowWithinProc(w.(*unit)); ok {
			return true
		}
	}
	return false
}
