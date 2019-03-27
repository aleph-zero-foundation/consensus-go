package alephzero

import "errors"

// A unit included in a poset.
type Unit interface {
	BaseUnit
	Height() int
	Parents() []Unit
	Level() int
}

// Returns the predecessor of a unit, i.e. the unit created by the same process that is one of this unit's parents.
func Predecessor(u Unit) (Unit, error) {
	pars := u.Parents()
	if len(pars) == 0 {
		return nil, errors.New("TODO: Make better error for parentless.")
	}
	return pars[0], nil
}

// Checks whether this unit is a prime unit.
func Prime(u Unit) bool {
	p, err := Predecessor(u)
	if err != nil {
		return true
	}
	return u.Level() > p.Level()
}
