package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/core-go/pkg/core"
)

type unit struct {
	creator   uint16
	epochID   gomel.EpochID
	height    int
	level     int
	version   int
	hash      gomel.Hash
	crown     gomel.Crown
	parents   []gomel.Unit
	floor     [][]gomel.Unit
	signature gomel.Signature
	data      core.Data
	rsData    []byte
}

// FromPreunit makes a new test unit based on the given preunit and parents.
func FromPreunit(pu gomel.Preunit, parents []gomel.Unit, dag gomel.Dag) gomel.Unit {
	var u unit
	u.parents = parents
	u.epochID = pu.EpochID()
	// Setting height, creator, signature, version, hash
	d := dag.(*Dag)
	setBasicInfo(&u, d, pu)
	setLevel(&u, d)
	setFloor(&u, d)
	return &u
}

func (u *unit) EpochID() gomel.EpochID {
	return u.epochID
}

func (u *unit) Floor(pid uint16) []gomel.Unit {
	return u.floor[pid]
}

func (u *unit) RandomSourceData() []byte {
	return u.rsData
}

func (u *unit) Data() core.Data {
	return u.data
}

func (u *unit) Creator() uint16 {
	return u.creator
}

func (u *unit) Signature() gomel.Signature {
	return u.signature
}

func (u *unit) Hash() *gomel.Hash {
	return &u.hash
}

func (u *unit) View() *gomel.Crown {
	return &u.crown
}

func (u *unit) Height() int {
	return u.height
}

func (u *unit) Parents() []gomel.Unit {
	return u.parents
}

func (u *unit) Level() int {
	return u.level
}

func (u *unit) AboveWithinProc(v gomel.Unit) bool {
	var w gomel.Unit
	for w = u; w != nil && w.Height() > v.Height(); w = gomel.Predecessor(w) {
	}
	if w == nil {
		return false
	}
	return *w.Hash() == *v.Hash()
}
