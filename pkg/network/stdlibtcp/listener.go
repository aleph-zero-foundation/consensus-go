package stdlibgo

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// listener object is responsible for listening for incoming synchronizations and handling them
type listener struct {
	channels []network.Channel
	poset    gomel.Poset
	postAdd  func()
	exitChan chan struct{}
	channels []network.Channel
}

func newListener(poset gomel.Poset, postAdd func(), chans []network.Channels) *listener {
	return &listener{
		channels: chans,
		poset:    poset,
		postAdd:  postAdd,
		exitChan: make(chan struct{}),
	}
}

func (l *listener) Start() {
	go l.main()
}

func (l *listener) Stop() {
	close(l.exitChan)
}

func (l *listener) ListenChannels() {
	return l.channels
}

func (l *listener) main() {
	for {
		select {
		case <-l.exitChan:
			return
		default:
			//do the job
		}
	}
}
