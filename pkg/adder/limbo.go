package adder

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// node is a struct that keeps a single preunit waiting to be added to dag.
type node struct {
	pu             gomel.Preunit
	dag            gomel.Dag
	missingParents int     // number of preunit's parents that we've never seen
	limboParents   int     // number of preunit's parents that are waiting in limbo
	children       []*node // list of other preunits in limbo that has this preunit as parent (maybe, because forks)
	wg             *sync.WaitGroup
	err            *error
}

// limbo is a buffer zone where preunits wait to be added to dag. A preunit with
// missing parents is waiting until all the parents are available. Then it's considered
// 'ready' and added to per-pid channel, from where it's picked by the worker doing Prepare+Insert.
// Only after that the preunit is removed from limbo.
type limbo struct {
	waiting     map[gomel.Hash]*node
	waitingByID map[uint64]*node
	missing     map[uint64][]*node
	readyNodes  []chan *node
	mx          sync.RWMutex
}

func newLimbo(ready []chan *node) *limbo {
	return &limbo{
		waiting:     make(map[gomel.Hash]*node),
		waitingByID: make(map[uint64]*node),
		missing:     make(map[uint64][]*node),
		readyNodes:  ready,
	}
}

func (l *limbo) get(hash *gomel.Hash) *node {
	l.mx.RLock()
	defer l.mx.RUnlock()
	return l.waiting[*hash]
}

func (l *limbo) checkIfReady(nd *node) {
	if nd.limboParents == 0 && nd.missingParents == 0 {
		l.readyNodes[nd.pu.Creator()] <- nd
	}
}

func (l *limbo) add(pu gomel.Preunit, dag gomel.Dag) *node {
	nd := &node{
		pu:  pu,
		dag: dag,
		wg:  new(sync.WaitGroup),
		err: new(error),
	}
	var unknown []uint64
	id := gomel.UnitID(pu, dag.NProc())
	l.mx.Lock()
	defer l.mx.Unlock()
	// find out which parents are in dag, which in limbo, and which are missing
	unknown = gomel.FindMissingParents(dag, pu.View())
	for _, unkID := range unknown {
		if par := l.waitingByID[unkID]; par != nil {
			nd.limboParents++
			par.children = append(par.children, nd)
		} else {
			nd.missingParents++
			if _, ok := l.missing[unkID]; !ok {
				l.missing[unkID] = make([]*node, 0, 8)
			}
			l.missing[unkID] = append(l.missing[unkID], nd)
		}
	}
	// add new node to limbo
	l.waiting[*pu.Hash()] = nd
	l.waitingByID[id] = nd
	// check if this unit is needed by something in limbo
	if children, ok := l.missing[id]; ok {
		nd.children = children
		for _, ch := range children {
			ch.missingParents--
			ch.limboParents++
		}
		delete(l.missing, id)
	} else {
		nd.children = make([]*node, 0, 8)
	}
	l.checkIfReady(nd)
	nd.wg.Add(1)
	return nd
}

func (l *limbo) remove(nd *node) {
	id := gomel.UnitID(nd.pu, uint16(len(l.readyNodes)))
	l.mx.Lock()
	defer l.mx.Unlock()
	delete(l.waiting, *(nd.pu.Hash()))
	delete(l.waitingByID, id)
	for _, ch := range nd.children {
		ch.limboParents--
		l.checkIfReady(ch)
	}
}
