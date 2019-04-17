package stdlibgo

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"golang.org/x/sync/semaphore"
	"time"
)

const (
	SYNC_DELAY = 1 // in seconds
)

type syncer struct {
	channels        []network.Channel
	channelSelector func() network.Channel
	poset           gomel.Poset
	postAdd         func()
	exitChan        chan struct{}
	syncSemaphore   *semaphore.Weighted
}

func newSyncer(chans []network.Channel, poset gomel.Poset, postAdd func(), maxSyncs int, channelSelector func() network.Channel) network.Syncer {
	return &syncer{
		channels:        chans,
		channelSelector: channelSelector,
		poset:           poset,
		postAdd:         postAdd,
		exitChan:        make(chan struct{}),
		syncSemaphore:   semaphore.NewWeighted(maxSyncs),
	}
}

func (s *syncer) Start() {
	go s.dispatcher()
}

func (s *syncer) Stop() {
	close(s.exitChan)
}

func (s *syncer) SyncChannels() []network.Channel {
	return s.channels
}

func (s *syncer) dispatcher() {
	for {
		select {
		case <-s.exitChan:
			return
		default:
			channel := s.channelSelector()
			go sync(channel)
			time.Sleep(SYNC_DELAY * time.Second)
		}
	}
}

func (s *syncer) sync(channel *channel) {
	if !channel.tryAcquire() {
		// channel already in use
		return
	}
	defer channel.release()

	if !s.syncSemaphore.TryAcquire(1) {
		// too many outgoing syncs
		return
	}
	defer s.syncSemaphore.Release(1)

	// TODO do the job
}
