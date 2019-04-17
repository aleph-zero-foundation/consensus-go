package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// Process is a top level object responsible for creating new units and
// exchanging them with other Processes
type Process struct {
	nProcesses int
	pid        int
	poset      gomel.Poset
	creator    *creator
	syncer     network.Syncer
	listener   network.Listener
}

func newProcess(n, pid int, poset gomel.Poset, creator *creator, syncer network.Syncer, listener network.Listener) *Process {
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
	p.creator.start()
	defer p.creator.stop()
	p.listener.Start()
	defer p.listener.Stop()
	p.syncer.Start()
	defer p.syncer.Stop()

	<-p.creator.done
}
