package unit

import (
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// unitInDag is a unit that is already inside the dag, and has all its properties precomputed and cached.
// It uses forking heights to optimize AboveWithinProc calls.
type unitInDag struct {
	gomel.Unit
	forkingHeight int
}

// Prepared transforms the given unit into unitInDag and computes forking height.
// The returned unit overrides AboveWithinProc method to use that forking height.
// NOTE: wrapping such a unit with any other unit wrapper will render forking height unaccessible.
func Prepared(u gomel.Unit, dag gomel.Dag) gomel.Unit {
	result := &unitInDag{u, math.MaxInt32}
	result.computeForkingHeight(dag)
	return result
}

func (u *unitInDag) computeForkingHeight(dag gomel.Dag) {
	// this implementation works as long as there is no race for writing/reading to dag.maxUnits, i.e.
	// as long as units created by one process are added atomically
	if gomel.Dealing(u) {
		if len(dag.MaximalUnitsPerProcess().Get(u.Creator())) > 0 {
			// this is a forking dealing unit
			u.forkingHeight = -1
		} else {
			u.forkingHeight = math.MaxInt32
		}
		return
	}
	if predecessor, ok := gomel.Predecessor(u).(*unitInDag); ok {
		found := false
		for _, v := range dag.MaximalUnitsPerProcess().Get(u.Creator()) {
			if v == predecessor {
				found = true
				break
			}
		}
		if found {
			u.forkingHeight = predecessor.forkingHeight
		} else {
			// there is already a unit that has 'predecessor' as a predecessor, hence u is a fork
			if predecessor.forkingHeight < predecessor.Height() {
				u.forkingHeight = predecessor.forkingHeight
			} else {
				u.forkingHeight = predecessor.Height()
			}
		}
	}
}
