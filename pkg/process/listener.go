package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// listener object is responsible for listening for incoming synchronizations and handling them
type listener struct {
	poset    gomel.Poset
	postAdd  func()
	exitChan chan struct{}
}

func newListener(poset gomel.Poset, postAdd func()) *listener {
	return &listener{
		poset:    poset,
		postAdd:  postAdd,
		exitChan: make(chan struct{}),
	}
}

func (l *listener) start() {
	go l.main()
}

func (l *listener) stop() {
	close(l.exitChan)
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
