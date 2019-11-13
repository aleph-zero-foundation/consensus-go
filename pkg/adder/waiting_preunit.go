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

// ready waitingPreunit is one without any waiting or missing parents.
func ready(wp *waitingPreunit) bool {
	return wp.waitingParents == 0 && wp.missingParents == 0
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
// which are waiting, and which are missing.
func (ad *adder) checkParents(wp *waitingPreunit) bool {
	missing := false
	unknown := gomel.FindMissingParents(ad.dag, wp.pu)
	for _, unkID := range unknown {
		if par, ok := ad.waitingByID[unkID]; ok {
			wp.waitingParents++
			par.children = append(par.children, wp)
		} else {
			missing = true
			wp.missingParents++
			ad.registerMissing(unkID, wp)
		}
	}
	return missing
}

// addOne takes a preunit for which some parents might be missing and puts
// it as a waitingPreunit in the buffer zone.
//
func (ad *adder) addOne(pu gomel.Preunit, source uint16) error {
	wp := &waitingPreunit{pu: pu, source: source}
	id := gomel.UnitID(pu)
	ad.mx.Lock()
	defer ad.mx.Unlock()
	if u := ad.dag.GetUnit(pu.Hash()); u != nil {
		return gomel.NewDuplicateUnit(u)
	}
	if wp, ok := ad.waiting[*pu.Hash()]; ok {
		return gomel.NewDuplicatePreunit(wp.pu)
	}
	if _, ok := ad.waitingByID[id]; ok {
		// We have a fork
		// SHALL BE DONE
		// Alert(fork, pu)
	}
	ad.waiting[*pu.Hash()] = wp
	ad.waitingByID[id] = wp
	if ad.checkParents(wp) {
		ad.resolveMissing(wp)
	}
	ad.checkIfMissing(wp, id)
	if ready(wp) {
		ad.sendToWorker(wp)
	}
	if wp.missingParents > 0 {
		return gomel.NewUnknownParents(wp.missingParents)
	}
	return nil
}

// addBatch adds a slice of preunits received from PID source to the buffer zone,
// using a single mutex lock for all of them. It does NOT check for missing parents,
// it assumes all preunits are sorted in topological order and can be added to the dag directly.
func (ad *adder) addBatch(preunits []gomel.Preunit, source uint16, errors []error) []error {
	var id uint64
	hashes := make([]*gomel.Hash, len(preunits))
	for i, pu := range preunits {
		hashes[i] = pu.Hash()
	}

	ad.mx.Lock()
	defer ad.mx.Unlock()
	alreadyInDag := ad.dag.GetUnits(hashes)
	for i, pu := range preunits {
		if alreadyInDag[i] != nil {
			errors[i] = gomel.NewDuplicateUnit(alreadyInDag[i])
			continue
		}
		if wp, ok := ad.waiting[*pu.Hash()]; ok {
			errors[i] = gomel.NewDuplicatePreunit(wp.pu)
			continue
		}
		id = gomel.UnitID(pu)
		if _, ok := ad.waitingByID[id]; ok {
			// We have a fork
			// SHALL BE DONE
			// Alert(fork, pu)
		}
		wp := &waitingPreunit{pu: pu, source: source}
		ad.waiting[*pu.Hash()] = wp
		ad.waitingByID[id] = wp
		ad.checkIfMissing(wp, id)
		ad.sendToWorker(wp)
	}
	return errors
}

// remove waitingPreunit from the buffer zone and notify its children.
func (ad *adder) remove(wp *waitingPreunit) {
	id := gomel.UnitID(wp.pu)
	ad.mx.Lock()
	defer ad.mx.Unlock()
	delete(ad.waiting, *(wp.pu.Hash()))
	delete(ad.waitingByID, id)
	for _, ch := range wp.children {
		ch.waitingParents--
		if ready(ch) {
			ad.sendToWorker(ch)
		}
	}
}
