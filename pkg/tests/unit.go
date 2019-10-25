package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unit struct {
	creator   uint16
	height    int
	level     int
	version   int
	hash      gomel.Hash
	crown     gomel.Crown
	parents   []gomel.Unit
	floor     [][]gomel.Unit
	signature gomel.Signature
	data      gomel.Data
	rsData    []byte
}

func (u *unit) Floor() [][]gomel.Unit {
	return u.floor
}

func (u *unit) RandomSourceData() []byte {
	return u.rsData
}

func (u *unit) Data() gomel.Data {
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

func (u *unit) Above(v gomel.Unit) bool {
	if u == nil || v == nil {
		return false
	}
	// BFS from u
	// If we need faster implementation we probably should use floors here
	seenUnits := make(map[gomel.Hash]bool)
	seenUnits[*u.Hash()] = true
	queue := []gomel.Unit{u}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		if w == v {
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

func (u *unit) AboveWithinProc(v gomel.Unit) bool {
	return u.Above(v)
}
