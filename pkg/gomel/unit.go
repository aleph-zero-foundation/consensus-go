package gomel

// Unit that belongs to the dag.
type Unit interface {
	BaseUnit
	// Parents of this unit.
	Parents() []Unit
	// Level of this unit in the dag, as defined in the Aleph protocol whitepaper.
	Level() int
	// AboveWithinProc checks if this unit is above the given unit produced by the same creator.
	AboveWithinProc(Unit) bool
	// Floor returns a slice of maximal units created by the given process that are strictly below this unit.
	Floor(uint16) []Unit
}

// Above checks if u is above v.
func Above(u, v Unit) bool {
	if v == nil || u == nil {
		return false
	}
	if SameUnit(u, v) {
		return true
	}
	for _, w := range u.Floor(v.Creator()) {
		if w.AboveWithinProc(v) {
			return true
		}
	}
	return false
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

// MaximalByPid computes all maximal units produced by pid present in parents and their floors.
// The result will be appended to the 'out' parameter.
func MaximalByPid(parents []Unit, pid uint16) []Unit {
	if parents[pid] == nil {
		return nil
	}
	startIx := 0
	maximal := []Unit{parents[pid]}
	for _, parent := range parents {
		if parent == nil {
			continue
		}
		for _, w := range parent.Floor(pid) {
			found, ri := false, -1
			for ix, v := range maximal[startIx:] {

				if Above(w, v) {
					found = true
					ri = ix
					// we can now break out of the loop since if we would find any other index for storing `w` it would be a
					// proof of self-forking
					break
				}

				if Above(v, w) {
					found = true
					// we can now break out of the loop since if `w` would be above some other index it would contradicts
					// the assumption that elements of `floors` (narrowed to some index) are not comparable
					break
				}

			}
			if !found {
				maximal = append(maximal, w)
			} else if ri >= 0 {
				maximal[startIx+ri] = w
			}
		}
	}
	return maximal
}

// HasForkingEvidence checks whether the unit is sufficient evidence of the given creator forking,
// i.e. it is above two units created by creator that share a predecessor.
func HasForkingEvidence(u Unit, creator uint16) bool {
	if Dealing(u) {
		return false
	}
	f := u.Floor(creator)
	return len(f) > 1 || (len(f) == 1 && !SameUnit(f[0], u.Parents()[creator]))
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
		if v != nil && Above(v, u) {
			return true
		}
	}
	return false
}

// ToHashes converts a list of units to a list of hashes.
func ToHashes(units []Unit) []*Hash {
	result := make([]*Hash, len(units))
	for i, u := range units {
		if u != nil {
			result[i] = u.Hash()
		}
	}
	return result
}
