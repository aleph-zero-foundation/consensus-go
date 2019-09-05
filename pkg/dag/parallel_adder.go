package dag

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type addRequest struct {
	pu gomel.Preunit
	cb gomel.Callback
}

type parallelDag struct {
	gomel.Dag
	sinks []chan addRequest
}

func (dag *parallelDag) AddUnit(pu gomel.Preunit, callback gomel.Callback) {
	if pu.Creator() < 0 || pu.Creator() >= dag.NProc() {
		callback(pu, nil, gomel.NewDataError("invalid creator"))
		return
	}
	dag.sinks[pu.Creator()] <- addRequest{
		pu: pu,
		cb: callback,
	}
}

type adderService struct {
	dag     gomel.Dag
	sources []chan addRequest
	tasks   sync.WaitGroup
}

func (as *adderService) adder(i uint16) {
	defer as.tasks.Done()
	for ar := range as.sources[i] {
		gomel.AddUnit(as.dag, ar.pu, ar.cb)
	}
}

func (as *adderService) Start() error {
	as.tasks.Add(int(as.dag.NProc()))
	for i := uint16(0); i < as.dag.NProc(); i++ {
		go as.adder(i)
	}
	return nil
}

func (as *adderService) Stop() {
	for _, c := range as.sources {
		close(c)
	}
	as.tasks.Wait()
}

// Parallelize adding units to the dag. Units created by a single process are still added consecutively.
//
// After the returned service has been stopped the dag will panic on attempts to add units.
func Parallelize(dag gomel.Dag) (gomel.Dag, process.Service) {
	nProc := dag.NProc()
	pipes := make([]chan addRequest, nProc)
	for i := range pipes {
		pipes[i] = make(chan addRequest, 10)
	}
	return &parallelDag{dag, pipes}, &adderService{dag: dag, sources: pipes}
}
