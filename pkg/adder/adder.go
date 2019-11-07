package adder

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// adder is a buffer zone where preunits wait to be added to dag. A preunit with
// missing parents is waiting until all the parents are available. Then it's considered
// 'ready' and added to per-pid channel, from where it's picked by the worker doing gomel.AddUnit.
type adder struct {
	dag         gomel.Dag
	decHandlers []gomel.DecodeErrorHandler
	chkHandlers []gomel.CheckErrorHandler
	keys        []gomel.PublicKey
	ready       []chan *node
	waiting     map[gomel.Hash]*node
	waitingByID map[uint64]*node
	missing     map[uint64][]*node
	mx          sync.Mutex
	wg          sync.WaitGroup
}

// New constructs a new adder that uses the given set of public keys to verify correctness of incoming preunits.
// Returns twice the same object implementing both gomel.Adder and gomel.Service.
func New(dag gomel.Dag, keys []gomel.PublicKey) (gomel.Adder, gomel.Service) {
	ready := make([]chan *node, dag.NProc())
	for i := range ready {
		ready[i] = make(chan *node, 32)
	}
	ad := &adder{
		dag:         dag,
		keys:        keys,
		ready:       ready,
		waiting:     make(map[gomel.Hash]*node),
		waitingByID: make(map[uint64]*node),
		missing:     make(map[uint64][]*node),
	}
	return ad, ad
}

func (ad *adder) AddDecodeErrorHandler(h gomel.DecodeErrorHandler) {
	ad.decHandlers = append(ad.decHandlers, h)
}

func (ad *adder) AddCheckErrorHandler(h gomel.CheckErrorHandler) {
	ad.chkHandlers = append(ad.chkHandlers, h)
}

func (ad *adder) AddUnit(pu gomel.Preunit, source uint16) error {
	// SHALL BE DONE: unit registry check here
	err := ad.checkCorrectness(pu)
	if err != nil {
		return err
	}
	return ad.addNode(pu)
}

func (ad *adder) AddUnits(preunits []gomel.Preunit, source uint16) *gomel.AggregateError {
	// SHALL BE DONE: unit registry check here
	errors := make([]error, len(preunits))
	for i, pu := range preunits {
		err := ad.checkCorrectness(pu)
		if err != nil {
			errors[i] = err
			preunits[i] = nil
		}
	}
	ad.addNodes(preunits, errors)
	return gomel.NewAggregateError(errors)
}

// Start the adding workers.
func (ad *adder) Start() error {
	ad.wg.Add(int(ad.dag.NProc()))
	for i := range ad.ready {
		go func(i int) {
			for nd := range ad.ready[i] {
				ad.handleReadyNode(nd)
			}
		}(i)
	}
	return nil
}

// Stop the adding workers.
func (ad *adder) Stop() {
	for _, c := range ad.ready {
		close(c)
	}
	ad.wg.Wait()
}

// handleReadyNode takes a node that was just picked from adder channel and performs gomel.AddUnit on it.
func (ad *adder) handleReadyNode(nd *node) {
	defer ad.remove(nd)
	parents, err := gomel.DecodeParents(ad.dag, nd.pu)
	if err != nil {
		for _, handler := range ad.decHandlers {
			if parents, err = handler(err); err == nil {
				break
			}
		}
		if err != nil {
			// log error
			return
		}
	}
	freeUnit := ad.dag.BuildUnit(nd.pu, parents)
	err = ad.dag.Check(freeUnit)
	if err != nil {
		for _, handler := range ad.chkHandlers {
			if err = handler(err); err == nil {
				break
			}
		}
		if err != nil {
			// log error
			return
		}
	}
	unitInDag := ad.dag.Transform(freeUnit)
	ad.dag.Insert(unitInDag)
	// log success

	// SHALL BE DONE: handle wrong control hash and ambiguous parents
	// ALSO SHALL BE DONE: some parents might be missing if node came from antichain sent by a malicious process
}

// checkCorrectness checks very basic correctness of the given preunit: creator and signature.
func (ad *adder) checkCorrectness(pu gomel.Preunit) error {
	if pu.Creator() >= ad.dag.NProc() {
		return gomel.NewDataError("invalid creator")
	}
	if ad.keys != nil && !ad.keys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("invalid signature")
	}
	return nil
}
