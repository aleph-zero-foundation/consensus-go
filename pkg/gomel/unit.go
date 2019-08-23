package gomel

import "errors"

// Unit that belongs to the dag.
type Unit interface {
	BaseUnit
	// Height of a unit is the length of the path between this unit and a dealing unit in the (induced) sub-dag containing all units produced by the same creator.
	Height() int
	// Parents of this unit, with predecessor being the first element of the returned slice.
	Parents() []Unit
	// Level of this unit in the dag, as defined in the Aleph protocol whitepaper.
	Level() int
	// Below checks if this unit is below the given unit.
	Below(Unit) bool
	// Floor returns a collection of units containing, for each process, all maximal units created by that process below the unit.
	Floor() [][]Unit
}

// CombineParentsFloorsPerProc combines floors of the provided parents just for a given creator.
// The result will be appended to the 'out' parameter.
func CombineParentsFloorsPerProc(parents []Unit, pid int, out *[]Unit) {

	startIx := len(*out)

	for _, parent := range parents {

		for _, w := range parent.Floor()[pid] {
			found, ri := false, -1
			for ix, v := range (*out)[startIx:] {

				if v.Below(w) {
					found = true
					ri = ix
					// we can now break out of the loop since if we would find any other index for storing `w` it would be a
					// proof of self-forking
					break
				}

				if w.Below(v) {
					found = true
					// we can now break out of the loop since if `w` would be above some other index it would contradicts
					// the assumption that elements of `floors` (narrowed to some index) are not comparable
					break
				}

			}
			if !found {
				*out = append(*out, w)
			} else if ri >= 0 {
				(*out)[startIx+ri] = w
			}
		}
	}
}

// HasSelfForkingEvidence returns true iff the given set of parents proves that the creator (that is parents[0].Creator())
// made a fork.
func HasSelfForkingEvidence(parents []Unit) bool {
	if len(parents) == 0 {
		return false
	}
	// using the knowledge of maximal units produced by 'creator' that are below some of the parents (their floor attributes),
	// check whether collection of these maximal units has a single maximal element
	var storage [1]Unit
	combinedFloor := storage[:0]
	CombineParentsFloorsPerProc(parents, parents[0].Creator(), &combinedFloor)
	if len(combinedFloor) > 1 {
		return true
	}
	// check if some other parent has an evidence of a unit made by 'creator' that is above our self-predecessor
	return *parents[0].Hash() != *combinedFloor[0].Hash()
}

// HasForkingEvidence checks whether the unit is sufficient evidence of the given creator forking,
// i.e. it is above two units created by creator that share a predecessor.
func HasForkingEvidence(u Unit, creator int) bool {
	if Dealing(u) {
		return false
	}
	if creator != u.Creator() {
		return len(u.Floor()[creator]) > 1
	}
	return HasSelfForkingEvidence(u.Parents())
}

// Predecessor of a unit is one of its parents, the one created by the same process as the given unit.
func Predecessor(u Unit) (Unit, error) {
	pars := u.Parents()
	if len(pars) == 0 {
		return nil, errors.New("no parents")
	}
	return pars[0], nil
}

// Prime checks whether the given unit is a prime unit.
func Prime(u Unit) bool {
	p, err := Predecessor(u)
	if err != nil {
		return true
	}
	return u.Level() > p.Level()
}

// Dealing checks if u is a dealing unit.
func Dealing(u Unit) bool {
	return len(u.Parents()) == 0
}

// BelowAny checks whether u is below any of the units in us.
func BelowAny(u Unit, us []Unit) bool {
	for _, v := range us {
		if v != nil && u.Below(v) {
			return true
		}
	}
	return false
}
