package growing

import (
	a "gitlab.com/alephledger/consensus-go/pkg"
)

type unit struct {
	creator int
	hash    a.Hash
	height  int
	parents []a.Unit
	level   int
	floor   []a.Unit
}

func newUnit(pu a.Preunit) *unit {
	return &unit{
		creator: pu.Creator(),
		hash:    *pu.Hash(),
	}
}

func (u *unit) Creator() int {
	return u.creator
}

func (u *unit) Hash() *a.Hash {
	return &u.hash
}

func (u *unit) Height() int {
	return u.height
}

func (u *unit) Parents() []a.Unit {
	return u.parents
}

func (u *unit) Level() int {
	return u.level
}

func (u *unit) setHeight(height int) {
	u.height = height
}

func (u *unit) addParent(parent a.Unit) {
	u.parents = append(u.parents, parent)
}

func (u *unit) setLevel(level int) {
	u.level = level
}

func (u *unit) computeFloor() {
	floors := [][]a.Unit{}
	for _, parent := range u.parents {
		if realParent, ok := parent.(*unit); ok {
			floors = append(floors, realParent.floor)
		} else {
			// TODO: this might be needed in the far future when there are special units that separate existing and nonexistent units
		}
	}
	u.floor = combineFloors(floors)
}

func combineFloors(floors [][]a.Unit) []a.Unit {
	// TODO: implement
	return []a.Unit{}
}
