package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type unit struct {
	creator   int
	height    int
	level     int
	version   int
	hash      gomel.Hash
	parents   []gomel.Unit
	floor     [][]gomel.Unit
	signature gomel.Signature
	data      []byte
	rsData    []byte
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

func (u *unit) Creator() int {
	return u.creator
}

func (u *unit) Signature() gomel.Signature {
	return u.signature
}

func (u *unit) Hash() *gomel.Hash {
	return &u.hash
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
			if _, exists := seenUnits[*wParent.Hash()]; !exists {
				queue = append(queue, wParent)
				seenUnits[*wParent.Hash()] = true
			}
		}
	}
	return false
}

func (u *unit) Above(v gomel.Unit) bool {
	return v.Below(u)
}

func (u *unit) HasForkingEvidence(creator int) bool {
	return false
}
