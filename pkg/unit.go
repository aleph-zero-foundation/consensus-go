package gomel

import "errors"

// Unit that belongs to the poset.
type Unit interface {
	BaseUnit
	// Height of a unit is the length of the path between this unit and a dealing unit in the (induced) sub-poset containing all units produced by the same creator.
	Height() int
	// Parents of this unit, with predecessor being the first element of returned slice.
	Parents() []Unit
	// Level of this unit in the poset, as defined in the Aleph protocol whitepaper.
	Level() int
	// Below tells if this unit is below the given unit.
	Below(Unit) bool
	// Above is a counterpart to Below.
	Above(Unit) bool
}

// Predecessor of a unit is one of its parents, the one created by the same process as the given unit.
func Predecessor(u Unit) (Unit, error) {
	pars := u.Parents()
	if len(pars) == 0 {
		return nil, errors.New("TODO: Make better error for parentless.")
	}
	return pars[0], nil
}

// Prime checks whether given unit is a prime unit.
func Prime(u Unit) bool {
	p, err := Predecessor(u)
	if err != nil {
		return true
	}
	return u.Level() > p.Level()
}
