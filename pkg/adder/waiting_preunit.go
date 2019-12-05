package adder

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
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

// checkIfMissing sets the children attribute of a newly created node, depending on if it was missing
func (ad *adder) checkIfMissing(wp *waitingPreunit) {
	if mp, ok := ad.missing[wp.id]; ok {
		wp.children = mp.neededBy
		for _, ch := range wp.children {
			ch.missingParents--
			ch.waitingParents++
		}
		delete(ad.missing, wp.id)
	} else {
		wp.children = []*waitingPreunit{}
	}
}

// checkParents finds out which parents of a newly created waitingPreunit are in dag,
// which are waiting, and which are missing.
func (ad *adder) checkParents(wp *waitingPreunit) {
	unknown := gomel.FindMissingParents(ad.dag, wp.pu)
	for _, unkID := range unknown {
		if par, ok := ad.waitingByID[unkID]; ok {
			wp.waitingParents++
			par.children = append(par.children, wp)
		} else {
			wp.missingParents++
			ad.registerMissing(unkID, wp)
		}
	}
}

// addPreunit as a waitingPreunit to the buffer zone.
// This method must be called under mutex!
func (ad *adder) addToWaiting(pu gomel.Preunit, source uint16) error {
	if wp, ok := ad.waiting[*pu.Hash()]; ok {
		return gomel.NewDuplicatePreunit(wp.pu)
	}
	id := gomel.UnitID(pu)
	if fork, ok := ad.waitingByID[id]; ok {
		ad.alert.NewFork(pu, fork.pu)
	}
	wp := &waitingPreunit{pu: pu, id: id, source: source}
	ad.waiting[*pu.Hash()] = wp
	ad.waitingByID[id] = wp
	ad.checkParents(wp)
	ad.checkIfMissing(wp)
	if wp.missingParents > 0 {
		ad.log.Debug().Int(logging.Height, wp.pu.Height()).Uint16(logging.Creator, wp.pu.Creator()).Uint16(logging.PID, wp.source).Int(logging.Size, wp.missingParents).Msg(logging.FetchParents)
		ad.fetchMissing(wp)
		return gomel.NewUnknownParents(wp.missingParents)
	}
	ad.sendIfReady(wp)
	return nil
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
