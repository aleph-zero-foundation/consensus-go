package unit

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type freeUnit struct {
	nProc     uint16
	creator   uint16
	signature gomel.Signature
	hash      gomel.Hash
	parents   []gomel.Unit
	crown     gomel.Crown
	data      gomel.Data
	rsData    []byte
	height    int
	level     int
	floor     map[uint16][]gomel.Unit
}

// New creates a new freeUnit based on the given preunit and a list of parents.
func New(pu gomel.Preunit, parents []gomel.Unit) gomel.Unit {
	return &freeUnit{
		nProc:     uint16(len(parents)),
		creator:   pu.Creator(),
		signature: pu.Signature(),
		crown:     *pu.View(),
		hash:      *pu.Hash(),
		parents:   parents,
		data:      pu.Data(),
		rsData:    pu.RandomSourceData(),
		height:    -1,
		level:     -1,
	}
}

func (u *freeUnit) RandomSourceData() []byte {
	return u.rsData
}

func (u *freeUnit) Data() gomel.Data {
	return u.data
}

func (u *freeUnit) Creator() uint16 {
	return u.creator
}

func (u *freeUnit) Signature() gomel.Signature {
	return u.signature
}

func (u *freeUnit) Hash() *gomel.Hash {
	return &u.hash
}

func (u *freeUnit) View() *gomel.Crown {
	return &u.crown
}

func (u *freeUnit) Parents() []gomel.Unit {
	return u.parents
}

func (u *freeUnit) Height() int {
	if u.height == -1 {
		u.computeHeight()
	}
	return u.height
}

func (u *freeUnit) computeHeight() {
	if gomel.Dealing(u) {
		u.height = 0
	} else {
		u.height = gomel.Predecessor(u).Height() + 1
	}
}

func (u *freeUnit) Level() int {
	if u.level == -1 {
		u.computeLevel()
	}
	return u.level
}

func (u *freeUnit) computeLevel() {
	u.level = gomel.LevelFromParents(u.parents)
}

func (u *freeUnit) Floor(pid uint16) []gomel.Unit {
	if u.floor == nil {
		u.computeFloor()
	}
	if fl, ok := u.floor[pid]; ok {
		return fl
	}
	if u.parents[pid] == nil {
		return nil
	}
	return u.parents[pid:(pid + 1)]
}

func (u *freeUnit) computeFloor() {
	u.floor = make(map[uint16][]gomel.Unit)
	if u.parents[u.creator] == nil { // this is a dealing unit
		return
	}
	for pid := uint16(0); pid < u.nProc; pid++ {
		maximal := gomel.MaximalByPid(u.parents, pid)
		if len(maximal) > 1 || (len(maximal) == 1 && !gomel.Equal(maximal[0], u.parents[pid])) {
			u.floor[pid] = maximal
		}
	}
}
