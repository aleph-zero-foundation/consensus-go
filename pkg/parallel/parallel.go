// Package parallel implements a service for adding units to a dag in parallel.
package parallel

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Parallel is a service for adding units to the poset parallelly.
// Units created by the same process will be added consecutively, even between different dags.
// The idea is to have multiple wrapers around a single base dag, and while the wrappers can be used by separate
// routines, they should stilluse one parallelization.
type Parallel struct {
	sources []<-chan addRequest
	sinks   []chan<- addRequest
	dags    []gomel.Dag
	wg      sync.WaitGroup
}

func (p *Parallel) initialize(nProc uint16) {
	sources := make([]<-chan addRequest, nProc)
	sinks := make([]chan<- addRequest, nProc)
	for i := range sinks {
		reqChan := make(chan addRequest, 10)
		sources[i] = reqChan
		sinks[i] = reqChan
	}
	p.sources = sources
	p.sinks = sinks
}

// Register a dag for which you would like an adder.
func (p *Parallel) Register(dag gomel.Dag) gomel.Adder {
	if p.dags == nil {
		p.initialize(uint16(dag.NProc()))
	}
	id := len(p.dags)
	p.dags = append(p.dags, dag)
	return &adder{id, p.sinks}
}

func (p *Parallel) adder(i int) {
	defer p.wg.Done()
	for req := range p.sources[i] {
		_, *req.err = gomel.AddUnit(p.dags[req.id], req.pu)
		req.wg.Done()
	}
}

// Start the adding routines.
func (p *Parallel) Start() error {
	p.wg.Add(len(p.sources))
	for i := range p.sources {
		go p.adder(i)
	}
	return nil
}

// Stop the adding routines. After this is called, any attempts to use adders created by this Parallel will result in a panic.
func (p *Parallel) Stop() {
	for _, c := range p.sinks {
		close(c)
	}
	p.wg.Wait()
}
