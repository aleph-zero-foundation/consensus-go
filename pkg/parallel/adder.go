// Package parallel implements a service for adding units to a dag in parallel.
package parallel

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type addRequest struct {
	pu  gomel.Preunit
	wg  *sync.WaitGroup
	err *error
}

type adder struct {
	requests []chan addRequest
	dag      gomel.Dag
	wg       sync.WaitGroup
}

// New parallel adder and the service that has to run for it to work.
// Units created by the same process will be added consecutively.
// Registering a dag after the service has been started, but before it was stopped results in undefined behaviour.
func New() (gomel.Adder, gomel.Service) {
	result := &adder{}
	return result, result
}

func (a *adder) initialize(nProc uint16) {
	a.requests = make([]chan addRequest, nProc)
	for i := range a.requests {
		a.requests[i] = make(chan addRequest, 10)
	}
}

func (a *adder) Register(dag gomel.Dag) {
	a.initialize(dag.NProc())
	a.dag = dag
}

func (a *adder) AddUnit(pu gomel.Preunit) error {
	if int(pu.Creator()) >= len(a.requests) {
		return gomel.NewDataError("invalid creator")
	}
	wg := &sync.WaitGroup{}
	var err error
	wg.Add(1)
	a.requests[pu.Creator()] <- addRequest{pu, wg, &err}
	wg.Wait()
	return err
}

func (a *adder) AddAntichain(preunits []gomel.Preunit) *gomel.AggregateError {
	wg := &sync.WaitGroup{}
	result := make([]error, len(preunits))
	wg.Add(len(preunits))
	for i, pu := range preunits {
		if int(pu.Creator()) >= len(a.requests) {
			result[i] = gomel.NewDataError("invalid creator")
			continue
		}
		a.requests[pu.Creator()] <- addRequest{pu, wg, &result[i]}
	}
	wg.Wait()
	return gomel.NewAggregateError(result)
}

func (a *adder) adder(i int) {
	defer a.wg.Done()
	for req := range a.requests[i] {
		_, *req.err = gomel.AddUnit(a.dag, req.pu)
		req.wg.Done()
	}
}

func (a *adder) Start() error {
	a.wg.Add(len(a.requests))
	for i := range a.requests {
		go a.adder(i)
	}
	return nil
}

func (a *adder) Stop() {
	for _, c := range a.requests {
		close(c)
	}
	a.wg.Wait()
}
