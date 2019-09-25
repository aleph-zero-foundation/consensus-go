package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unit struct {
	creator     uint16
	height      int
	level       int
	version     int
	hash        gomel.Hash
	controlHash gomel.Hash
	parents     []gomel.Unit
	floor       [][]gomel.Unit
	signature   gomel.Signature
	data        []byte
	rsData      []byte
}

func (u *unit) Floor() [][]gomel.Unit {
	return u.floor
}

func (u *unit) RandomSourceData() []byte {
	return u.rsData
}

func (u *unit) Data() []byte {
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

func (u *unit) ControlHash() *gomel.Hash {
	return &u.controlHash
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

func (u *unit) Below(v gomel.Unit) bool {
	if v == nil {
		return false
	}
	// BFS from v
	// If we need faster implementation we probably should use floors here
	seenUnits := make(map[gomel.Hash]bool)
	seenUnits[*v.Hash()] = true
	queue := []gomel.Unit{v}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if *w.Hash() == *u.Hash() {
			return true
		}
		for _, wParent := range w.Parents() {
			if wParent == nil {
				continue
			}
			if _, exists := seenUnits[*wParent.Hash()]; !exists {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	return false
}

func (u *unit) HasForkingEvidence(creator uint16) bool {
	return false
}
