package gomel

// Unit that belongs to the dag.
type Unit interface {
	BaseUnit
	// Parents of this unit.
	Parents() []Unit
	// Level of this unit in the dag, as defined in the Aleph protocol whitepaper.
	Level() int
	// Above checks if this unit is above the given unit.
	Above(Unit) bool
	// Floor returns a collection of units containing, for each process, all maximal units created by that process below the unit.
	Floor() [][]Unit
}

// LevelFromParents calculates level of a unit having given set of parents.
func LevelFromParents(parents []Unit) int {
	nProc := uint16(len(parents))
	level := 0
	onLevel := uint16(0)
	for _, p := range parents {
		if p == nil {
			continue
		}
		if p.Level() == level {
			onLevel++
		} else if p.Level() > level {
			onLevel = 1
			level = p.Level()
		}
	}
	if IsQuorum(nProc, onLevel) {
		level++
	}
	return level
}

// CombineParentsFloorsPerProc combines floors of the provided parents just for a given creator.
// The result will be appended to the 'out' parameter.
func CombineParentsFloorsPerProc(parents []Unit, pid uint16, out *[]Unit) {

	startIx := len(*out)

	for _, parent := range parents {
		if parent == nil {
			continue
		}
		for _, w := range parent.Floor()[pid] {
			found, ri := false, -1
			for ix, v := range (*out)[startIx:] {

				if w.Above(v) {
					found = true
					ri = ix
					// we can now break out of the loop since if we would find any other index for storing `w` it would be a
					// proof of self-forking
					break
				}

				if v.Above(w) {
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

// HasSelfForkingEvidence returns true iff the given set of parents proves that the creator
// made a fork.
func HasSelfForkingEvidence(parents []Unit, creator uint16) bool {
	if parents[creator] == nil {
		return false
	}
	// using the knowledge of maximal units produced by 'creator' that are below some of the parents (their floor attributes),
	// check whether collection of these maximal units has a single maximal element
	var storage [1]Unit
	combinedFloor := storage[:0]
	CombineParentsFloorsPerProc(parents, creator, &combinedFloor)
	if len(combinedFloor) > 1 {
		return true
	}
	// check if some other parent has an evidence of a unit made by 'creator' that is above our self-predecessor
	return *parents[creator].Hash() != *combinedFloor[0].Hash()
}

// HasForkingEvidence checks whether the unit is sufficient evidence of the given creator forking,
// i.e. it is above two units created by creator that share a predecessor.
func HasForkingEvidence(u Unit, creator uint16) bool {
	if Dealing(u) {
		return false
	}
	if creator != u.Creator() {
		return len(u.Floor()[creator]) > 1
	}
	return HasSelfForkingEvidence(u.Parents(), creator)
}

// Prime checks whether the given unit is a prime unit.
func Prime(u Unit) bool {
	p := Predecessor(u)
	return (p == nil) || u.Level() > p.Level()
}

// Predecessor of a unit is one of its parents, the one created by the same process as the given unit.
func Predecessor(u Unit) Unit {
	return u.Parents()[u.Creator()]
}

// Dealing checks if u is a dealing unit.
func Dealing(u Unit) bool {
	return Predecessor(u) == nil
}

// BelowAny checks whether u is below any of the units in us.
func BelowAny(u Unit, us []Unit) bool {
	for _, v := range us {
		if v != nil && v.Above(u) {
			return true
		}
	}
	return false
}
