package adder

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// waitingPreunit is a struct that keeps a single preunit waiting to be added to dag.
type waitingPreunit struct {
	pu             gomel.Preunit
	source         uint16            // pid of the process that sent us this preunit
	missingParents int               // number of preunit's parents that we've never seen
	waitingParents int               // number of preunit's parents that are waiting in adder
	children       []*waitingPreunit // list of other preunits that has this preunit as parent
}

// checkIfMissing sets the children attribute of a newly created node, depending on if it was missing
func (ad *adder) checkIfMissing(wp *waitingPreunit, id uint64) {
	if mp, ok := ad.missing[id]; ok {
		wp.children = mp.neededBy
		for _, ch := range wp.children {
			ch.missingParents--
			ch.waitingParents++
		}
		delete(ad.missing, id)
	} else {
		wp.children = make([]*waitingPreunit, 0, 8)
	}
}

// checkParents finds out which parents of a newly created waitingPreunit are in dag,
// which are waiting, and which are missing. Returns the number of missing parents.
func (ad *adder) checkParents(wp *waitingPreunit) int {
	unknown := gomel.FindMissingParents(ad.dag, wp.pu) // SHALL BE DONE: Return not only parents, but also units below them
	for _, unkID := range unknown {
		if par, ok := ad.waitingByID[unkID]; ok {
			wp.waitingParents++
			par.children = append(par.children, wp)
		} else {
			wp.missingParents++
			ad.registerMissing(unkID, wp)
		}
	}
	return wp.missingParents
}

// addPreunit as a waitingPreunit to the buffer zone.
// This method must be called under mutex!
func (ad *adder) addToWaiting(pu gomel.Preunit, source uint16) error {
	if wp, ok := ad.waiting[*pu.Hash()]; ok {
		return gomel.NewDuplicatePreunit(wp.pu)
	}
	id := gomel.UnitID(pu)
	if _, ok := ad.waitingByID[id]; ok {
		// We have a fork
		// SHALL BE DONE
		// Alert(fork, pu)
	}
	wp := &waitingPreunit{pu: pu, source: source}
	ad.waiting[*pu.Hash()] = wp
	ad.waitingByID[id] = wp
	if ad.checkParents(wp) > 0 {
		ad.resolveMissing(wp)
	}
	ad.checkIfMissing(wp, id)
	ad.checkIfReady(wp)
	if wp.missingParents > 0 {
		return gomel.NewUnknownParents(wp.missingParents)
	}
	return nil
}

// remove waitingPreunit from the buffer zone and notify its children.
// Must be called under mutex.
func (ad *adder) remove(wp *waitingPreunit) {
	id := gomel.UnitID(wp.pu)
	ad.mx.Lock()
	defer ad.mx.Unlock()
	delete(ad.waiting, *(wp.pu.Hash()))
	delete(ad.waitingByID, id)
	for _, ch := range wp.children {
		ch.waitingParents--
		ad.checkIfReady(ch)
	}
}
