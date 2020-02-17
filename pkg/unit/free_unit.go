package unit

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/core"
)

type freeUnit struct {
	gomel.Preunit
	parents []gomel.Unit
	level   int
	floor   map[uint16][]gomel.Unit
}

// New constructs a new freeUnit with given set of parents and signs it with provided private key.
func New(creator uint16, epoch gomel.EpochID, parents []gomel.Unit, level int, data core.Data, rsData []byte, pk gomel.PrivateKey) gomel.Unit {
	crown := gomel.CrownFromParents(parents)
	height := crown.Heights[creator] + 1
	id := gomel.ID(height, creator, epoch)
	hash := ComputeHash(id, crown, data, rsData)
	signature := pk.Sign(hash)
	u := &freeUnit{
		Preunit: &preunit{creator, epoch, height, signature, hash, crown, data, rsData},
		parents: parents,
		level:   level,
	}
	u.computeFloor()
	return u
}

// FromPreunit creates a new freeUnit based on the given preunit and a list of parents.
func FromPreunit(pu gomel.Preunit, parents []gomel.Unit) gomel.Unit {
	u := &freeUnit{
		Preunit: pu,
		parents: parents,
		level:   gomel.LevelFromParents(parents),
	}
	u.computeFloor()
	return u
}

func (u *freeUnit) Parents() []gomel.Unit {
	return u.parents
}

func (u *freeUnit) Level() int {
	return u.level
}

func (u *freeUnit) Floor(pid uint16) []gomel.Unit {
	if fl, ok := u.floor[pid]; ok {
		return fl
	}
	if u.parents[pid] == nil {
		return nil
	}
	return u.parents[pid:(pid + 1)]
}

func (u *freeUnit) AboveWithinProc(v gomel.Unit) bool {
	if u.Creator() != v.Creator() {
		return false
	}
	var w gomel.Unit
	for w = u; w != nil && w.Height() > v.Height(); w = gomel.Predecessor(w) {
	}
	if w == nil {
		return false
	}
	return *w.Hash() == *v.Hash()
}

func (u *freeUnit) computeFloor() {
	u.floor = make(map[uint16][]gomel.Unit)
	if gomel.Dealing(u) {
		return
	}
	for pid := uint16(0); pid < uint16(len(u.parents)); pid++ {
		maximal := gomel.MaximalByPid(u.parents, pid)
		if len(maximal) > 1 || (len(maximal) == 1 && !gomel.Equal(maximal[0], u.parents[pid])) {
			u.floor[pid] = maximal
		}
	}
}
