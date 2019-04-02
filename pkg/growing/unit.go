package growing

import gomel "gitlab.com/alephledger/consensus-go/pkg"

type unit struct {
	creator int
	hash    gomel.Hash
	height  int
	parents []gomel.Unit
	level   int
	floor   [][]gomel.Unit
}

func newUnit(pu gomel.Preunit) *unit {
	return &unit{
		creator: pu.Creator(),
		hash:    *pu.Hash(),
	}
}

// Returns the creator id of the unit.
func (u *unit) Creator() int {
	return u.creator
}

// Returns the hash of the unit.
func (u *unit) Hash() *gomel.Hash {
	return &u.hash
}

// How many units created by the same process are below the unit.
func (u *unit) Height() int {
	return u.height
}

// Returns the parents of the unit.
func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

// Returns the level of the unit.
func (u *unit) Level() int {
	return u.level
}

func (u *unit) setHeight(height int) {
	u.height = height
}

func (u *unit) addParent(parent gomel.Unit) {
	u.parents = append(u.parents, parent)
}

func (u *unit) setLevel(level int) {
	u.level = level
}

func (u *unit) computeFloor() {
	floors := [][][]gomel.Unit{}
	for _, parent := range u.parents {
		if realParent, ok := parent.(*unit); ok {
			floors = append(floors, realParent.floor)
		} else {
			// TODO: this might be needed in the far future when there are special units that separate existing and nonexistent units
		}
	}
	u.floor = combineFloors(floors)
}

func combineFloors(floors [][][]gomel.Unit) [][]gomel.Unit {
	// TODO: implement
	return [][]gomel.Unit{}
}
