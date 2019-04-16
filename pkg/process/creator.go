package process

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Creator is responsible for creating and adding units to a poset.
// After meeting conditions given by stopCondition it lets know by done channel.
type creator struct {
	poset         gomel.Poset
	postAdd       func()
	stopCondition func() bool
	exitChan      chan struct{}
	done          chan struct{}
}

func newCreator(poset gomel.Poset, postAdd func(), stopCond func() bool, done chan struct{}) *creator {
	return &creator{
		poset:         poset,
		postAdd:       postAdd,
		stopCondition: stopCond,
		exitChan:      make(chan struct{}),
		done:          done,
	}
}

func (c *creator) start() {
	go c.main()
}

func (c *creator) stop() {
	close(c.exitChan)
}

func (c *creator) main() {
	for {
		select {
		case <-c.exitChan:
			return
		default:
			//do the job
			if c.stopCondition() {
				close(c.done)
				return
			}
		}
	}
}
