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
	chanServ   network.ConnectionServer
	syncer     network.Syncer
}

func NewProcess(n, pid int, poset gomel.Poset, creator *creator, connServ network.ConnectionServer, syncer network.Syncer) *Process {
	newProc := &Process{
		nProcesses: n,
		pid:        pid,
		poset:      poset,
		creator:    creator,
		chanServ:   chanServ,
		syncer:     syncer,
	}
	return newProc
}

func (p *Process) Run() {
	p.creator.start()
	defer p.creator.stop()
	p.chanServ.Listen()
	p.chanServ.Dial()
	defer p.chanServ.Stop()
	p.syncer.Start()
	defer p.syncer.Stop()

	<-p.creator.done
}
