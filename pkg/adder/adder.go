package adder

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/dag/unit"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type adder struct {
	nProc uint16
	keys  []gomel.PublicKey
	limbo *limbo
	ready []chan *node
	wg    sync.WaitGroup
}

// New constructs a new adder that uses the given set of public keys to verify correctness of incoming preunits.
// Returns twice the same object implementing both gomel.Adder and gomel.Service.
func New(keys []gomel.PublicKey) (gomel.Adder, gomel.Service) {
	nProc := uint16(len(keys))
	ready := make([]chan *node, nProc)
	for i := range ready {
		ready[i] = make(chan *node, 32)
	}
	ad := &adder{
		nProc: nProc,
		keys:  keys,
		limbo: newLimbo(ready),
		ready: ready,
	}
	return ad, ad
}

func (ad *adder) AddAntichain(preunits []gomel.Preunit, dag gomel.Dag) *gomel.AggregateError {
	return nil
}

func (ad *adder) AddUnit(pu gomel.Preunit, dag gomel.Dag) error {
	err := ad.checkCorrectness(pu)
	if err != nil {
		return err
	}
	// check if we've already seen this unit. We remove units from limbo AFTER adding them to dag,
	// so checking limbo first is required to ensure thread safety.
	if nd := ad.limbo.get(pu.Hash()); nd != nil {
		nd.wg.Wait()
		return *(nd.err)
	}
	if u := dag.GetUnit(pu.Hash()); u != nil {
		return gomel.NewDuplicateUnit(u)
	}
	nd := ad.limbo.add(pu, dag)
	nd.wg.Wait()
	return *(nd.err)
}

// Start the adding workers.
func (ad *adder) Start() error {
	ad.wg.Add(int(ad.nProc))
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

// handleReadyNode takes a limbo node that was just picked from adder channel and performs Prepare+Insert on it.
func (ad *adder) handleReadyNode(nd *node) {
	defer nd.wg.Done()
	defer ad.limbo.remove(nd) // TODO maybe not remove on every error...
	parents, err := gomel.GetByCrown(nd.dag, nd.pu.View())
	if err != nil {
		// TO BE DONE handle wrong control hash and ambiguous parents
		*nd.err = err
		return
	}
	freeUnit := unit.New(nd.pu, parents)
	unitInDag, err := nd.dag.Prepare(freeUnit)
	if err != nil {
		*nd.err = err
		return
	}
	nd.dag.Insert(unitInDag)
}

// checkCorrectness checks very basic correctness of the given preunit: creator and signature.
func (ad *adder) checkCorrectness(pu gomel.Preunit) error {
	if pu.Creator() >= ad.nProc {
		return gomel.NewDataError("invalid creator")
	}
	if !ad.keys[pu.Creator()].Verify(pu) {
		return gomel.NewDataError("invalid signature")
	}
	return nil
}
