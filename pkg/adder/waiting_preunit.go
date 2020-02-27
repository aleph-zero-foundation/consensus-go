package adder

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// waitingPreunit is a struct that keeps a single preunit waiting to be added to dag.
type waitingPreunit struct {
	pu             gomel.Preunit
	id             uint64
	source         uint16            // pid of the process that sent us this preunit
	missingParents int               // number of preunit's parents that we've never seen
	waitingParents int               // number of preunit's parents that are waiting in adder
	children       []*waitingPreunit // list of other preunits that has this preunit as parent
	failed         bool              // flag for signaling problems with adding this unit
}

// checkIfMissing sets the children attribute of a newly created waitingPreunit, depending on if it was missing
func (ad *adder) checkIfMissing(wp *waitingPreunit) {
	if mp, ok := ad.missing[wp.id]; ok {
		wp.children = mp.neededBy
		for _, ch := range wp.children {
			ch.missingParents--
			ch.waitingParents++
		}
		delete(ad.missing, wp.id)
	} else {
		wp.children = make([]*waitingPreunit, 0, 8)
	}
}

// checkParents finds out which parents of a newly created waitingPreunit are in the dag,
// which are waiting, and which are missing. Sets values of waitingParents and missingParents
// accordingly. Additionally, returns maximal heights of dag.
func (ad *adder) checkParents(wp *waitingPreunit) []int {
	epoch := wp.pu.EpochID()
	maxHeights := gomel.MaxView(ad.dag).Heights
	for creator, height := range wp.pu.View().Heights {
		if height > maxHeights[creator] {
			parentID := gomel.ID(height, uint16(creator), epoch)
			if par, ok := ad.waitingByID[parentID]; ok {
				wp.waitingParents++
				par.children = append(par.children, wp)
			} else {
				wp.missingParents++
				ad.registerMissing(parentID, wp)
			}
		}
	}
	return maxHeights
}

// remove waitingPreunit from the buffer zone and notify its children.
func (ad *adder) remove(wp *waitingPreunit) {
	ad.mx.Lock()
	defer ad.mx.Unlock()
	if wp.failed {
		ad.removeFailed(wp)
	} else {
		delete(ad.waiting, *(wp.pu.Hash()))
		delete(ad.waitingByID, wp.id)
		for _, ch := range wp.children {
			ch.waitingParents--
			ad.sendIfReady(ch)
		}
	}
}

// removeFailed removes from the buffer zone a ready preunit which we failed to add, together with all its descendants.
func (ad *adder) removeFailed(wp *waitingPreunit) {
	delete(ad.waiting, *(wp.pu.Hash()))
	delete(ad.waitingByID, wp.id)
	for _, ch := range wp.children {
		ad.removeFailed(ch)
	}
}
