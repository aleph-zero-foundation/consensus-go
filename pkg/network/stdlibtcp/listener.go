package stdlibgo

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"

	"golang.org/x/sync/semaphore"
)

// listener object is responsible for listening for incoming synchronizations and handling them
type listener struct {
	channels        []network.Channel
	poset           gomel.Poset
	postAdd         func()
	exitChan        chan struct{}
	listenSemaphore *semaphore.Weighted
}

func newListener(poset gomel.Poset, postAdd func(), chans []network.Channels, maxSyncs int) network.Listener {
	return &listener{
		channels:        chans,
		poset:           poset,
		postAdd:         postAdd,
		exitChan:        make(chan struct{}),
		listenSemaphore: semaphore.NewWeighted(maxSyncs),
	}
}

func (l *listener) Start() {
	for _, channel := range l.channels {
		go l.sync(channel)
	}
}

func (l *listener) Stop() {
	close(l.exitChan)
}

func (l *listener) ListenChannels() []network.Channel {
	return l.channels
}

func (l *listener) sync(channel network.Channel) {
	for {
		select {
		case <-l.exitChan:
			return
		default:
			if !channel.tryAcquire() {
				// channel already in use
				return
			}
			defer channel.release()

			if !l.listenSemaphore.TryAcquire(1) {
				// too many incomming syncs
				return
			}
			defer l.syncSem.Release(1)

			// TODO do the job
		}
	}
}
