package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type syncer struct {
	poset    gomel.Poset
	postAdd  func()
	exitChan chan struct{}
}

func newSyncer(poset gomel.Poset, postAdd func()) *syncer {
	return &syncer{
		poset:    poset,
		postAdd:  postAdd,
		exitChan: make(chan struct{}),
	}
}

func (s *syncer) start() {
	go s.main()
}

func (s *syncer) stop() {
	close(s.exitChan)
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
