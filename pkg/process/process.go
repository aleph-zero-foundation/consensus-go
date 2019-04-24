package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NOTE this is a simple wrapper showcasing how to use creator, network.ConnectionServer,
// and sync.Server together. It will be removed in the future and its code moved to cmd.
// Process is a top level object responsible for creating new units and
// exchanging them with other Processes
type Process struct {
	nProcesses int
	pid        int
	poset      gomel.Poset
	creator    *creator
	connServ   network.ConnectionServer
	server     sync.Server
}

func NewProcess(creator *creator, connServ network.ConnectionServer, server sync.Server) *Process {
	newProc := &Process{
		creator:  creator,
		connServ: connServ,
		server:   server,
	}
	return newProc
}

func (p *Process) Run() {
	p.creator.start()
	defer p.creator.stop()
	p.connServ.Listen()
	p.connServ.Dial()
	defer p.connServ.Stop()
	p.server.Start()
	defer p.server.Stop()

	<-p.creator.done
}
