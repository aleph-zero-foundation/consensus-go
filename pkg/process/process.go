package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Process is a top level object responsible for creating new units and
// exchanging them with other Processes
type Process struct {
	nProcesses int
	pid        int
	poset      gomel.Poset
	creator    *creator
	syncer     *syncer
	listener   *listener
}

func newProcess(n, pid int, poset gomel.Poset, creator *creator, syncer *syncer, listener *listener) *Process {
	newProc := &Process{
		nProcesses: n,
		pid:        pid,
		poset:      poset,
		creator:    creator,
		syncer:     syncer,
		listener:   listener,
	}
	return newProc
}

func (p *Process) run() {
	p.listener.start()
	defer p.listener.stop()
	p.creator.start()
	defer p.creator.stop()
	p.syncer.start()
	defer p.syncer.stop()

	<-p.creator.done
}
