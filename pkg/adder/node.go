package adder

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// node is a struct that keeps a single preunit waiting to be added to dag.
type node struct {
	pu             gomel.Preunit
	missingParents int     // number of preunit's parents that we've never seen
	limboParents   int     // number of preunit's parents that are waiting in limbo
	children       []*node // list of other preunits in limbo that has this preunit as parent (maybe, because forks)
	err            error
}

func (ad *adder) checkIfReady(nd *node) {
	if nd.limboParents == 0 && nd.missingParents == 0 {
		ad.ready[nd.pu.Creator()] <- nd
	}
}

// checkIfMissing sets the children attribute of a newly created node, depending on if it was missing
func (ad *adder) checkIfMissing(nd *node, id uint64) {
	if children, ok := ad.missing[id]; ok {
		nd.children = children
		for _, ch := range children {
			ch.missingParents--
			ch.limboParents++
		}
		delete(ad.missing, id)
	} else {
		nd.children = make([]*node, 0, 8)
	}
}

func (ad *adder) addNode(pu gomel.Preunit) error {
	nd := &node{pu: pu}
	id := gomel.UnitID(pu)
	ad.mx.Lock()
	defer ad.mx.Unlock()
	if u := ad.dag.GetUnit(pu.Hash()); u != nil {
		return gomel.NewDuplicateUnit(u)
	}
	if nd, ok := ad.waiting[*pu.Hash()]; ok {
		return gomel.NewDuplicatePreunit(nd.pu)
	}
	if _, ok := ad.waitingByID[id]; ok {
		// We have a fork
		// SHALL BE DONE
		// Alert(fork, pu)
	}
	// find out which parents are in dag, which in waiting, and which are missing
	unknown := gomel.FindMissingParents(ad.dag, pu.View())
	for _, unkID := range unknown {
		if par, ok := ad.waitingByID[unkID]; ok {
			nd.limboParents++
			par.children = append(par.children, nd)
		} else {
			nd.missingParents++
			if _, ok := ad.missing[unkID]; !ok {
				ad.missing[unkID] = make([]*node, 0, 8)
			}
			ad.missing[unkID] = append(ad.missing[unkID], nd)
		}
	}
	ad.waiting[*pu.Hash()] = nd
	ad.waitingByID[id] = nd
	ad.checkIfMissing(nd, id)
	ad.checkIfReady(nd)
	if nd.missingParents > 0 {
		return gomel.NewUnknownParents(nd.missingParents)
	}
	return nil
}

// addNodes does NOT check for missing parents, it assumes all preunits
// are sorted in topological order and can be added to the dag directly.
func (ad *adder) addNodes(preunits []gomel.Preunit, errors []error) {
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
		if nd, ok := ad.waiting[*pu.Hash()]; ok {
			errors[i] = gomel.NewDuplicatePreunit(nd.pu)
			continue
		}
		id = gomel.UnitID(pu)
		if _, ok := ad.waitingByID[id]; ok {
			// We have a fork
			// SHALL BE DONE
			// Alert(fork, pu)
		}
		nd := &node{pu: pu}
		ad.waiting[*pu.Hash()] = nd
		ad.waitingByID[id] = nd
		ad.checkIfMissing(nd, id)
		ad.ready[pu.Creator()] <- nd
	}
}

func (ad *adder) remove(nd *node) {
	id := gomel.UnitID(nd.pu)
	ad.mx.Lock()
	defer ad.mx.Unlock()
	delete(ad.waiting, *(nd.pu.Hash()))
	delete(ad.waitingByID, id)
	for _, ch := range nd.children {
		ch.limboParents--
		ad.checkIfReady(ch)
	}
}
