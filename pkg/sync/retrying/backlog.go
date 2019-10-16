package retrying

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type backlog struct {
	sync.Mutex
	backlog map[gomel.Hash]gomel.Preunit
}

func newBacklog() *backlog {
	return &backlog{
		backlog: make(map[gomel.Hash]gomel.Preunit),
	}
}

func (b *backlog) add(pu gomel.Preunit) bool {
	b.Lock()
	defer b.Unlock()
	if _, ok := b.backlog[*pu.Hash()]; ok {
		return false
	}
	b.backlog[*pu.Hash()] = pu
	return true
}

func (b *backlog) del(h *gomel.Hash) {
	b.Lock()
	defer b.Unlock()
	delete(b.backlog, *h)
}

func (b *backlog) get(h *gomel.Hash) gomel.Preunit {
	b.Lock()
	defer b.Unlock()
	return b.backlog[*h]
}

func (b *backlog) refallback(findOut func(pu gomel.Preunit)) {
	b.Lock()
	defer b.Unlock()
	for _, pu := range b.backlog {
		findOut(pu)
	}
}
