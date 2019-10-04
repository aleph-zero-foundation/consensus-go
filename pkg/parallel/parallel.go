// Package parallel implements a service for adding units to a dag in parallel.
package parallel

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Parallel is a service for adding units to the dag in parallel.
// Units created by the same process will be added consecutively, even between different dags.
// The idea is to have multiple wrappers around a single base dag, and while the wrappers can be used by separate
// routines, they should still use one parallelization.
type Parallel struct {
	reqChans []chan addRequest
	dags     []gomel.Dag
	wg       sync.WaitGroup
}

func (p *Parallel) initialize(nProc uint16) {
	p.reqChans = make([]chan addRequest, nProc)
	for i := range p.reqChans {
		p.reqChans[i] = make(chan addRequest, 10)
	}
}

// Register a dag for which you would like an adder.
func (p *Parallel) Register(dag gomel.Dag) gomel.Adder {
	if p.dags == nil {
		p.initialize(uint16(dag.NProc()))
	}
	dagID := len(p.dags)
	p.dags = append(p.dags, dag)
	return &adder{dagID, p.reqChans}
}

func (p *Parallel) adder(i int) {
	defer p.wg.Done()
	for req := range p.reqChans[i] {
		_, *req.err = gomel.AddUnit(p.dags[req.dagID], req.pu)
		req.wg.Done()
	}
}

// Start the adding routines.
func (p *Parallel) Start() error {
	p.wg.Add(len(p.reqChans))
	for i := range p.reqChans {
		go p.adder(i)
	}
	return nil
}

// Stop the adding routines. After this is called, any attempts to use adders created by this Parallel will result in a panic.
func (p *Parallel) Stop() {
	for _, c := range p.reqChans {
		close(c)
	}
	p.wg.Wait()
}
