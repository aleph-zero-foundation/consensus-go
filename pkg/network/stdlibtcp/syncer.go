package stdlibgo

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type syncer struct {
	channels []network.Channel
	poset    gomel.Poset
	postAdd  func()
	exitChan chan struct{}
}

func newSyncer(chans []network.Channel, poset gomel.Poset, postAdd func()) network.Syncer {
	return &syncer{
		channels: chans,
		poset:    poset,
		postAdd:  postAdd,
		exitChan: make(chan struct{}),
	}
}

func (s *syncer) Start() {
	go s.main()
}

func (s *syncer) Stop() {
	close(s.exitChan)
}

func (s *syncer) SyncChannels() []network.Channel {
	return s.channels
}

func (s *syncer) main() {
	for {
		select {
		case <-s.exitChan:
			return
		default:
			//do the job
		}
	}
}
