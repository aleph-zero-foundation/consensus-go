package growing

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

type tcBag struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.ThresholdCoin
}

func newTcBag() *tcBag {
	return &tcBag{contents: map[gomel.Hash]*tcoin.ThresholdCoin{}}
}

func (tcs *tcBag) remove(h *gomel.Hash) {
	tcs.Lock()
	defer tcs.Unlock()
	delete(tcs.contents, *h)
}

func (tcs *tcBag) add(h *gomel.Hash, tc *tcoin.ThresholdCoin) {
	tcs.Lock()
	defer tcs.Unlock()
	tcs.contents[*h] = tc
}

func (tcs *tcBag) get(h *gomel.Hash) *tcoin.ThresholdCoin {
	tcs.RLock()
	defer tcs.RUnlock()
	return tcs.contents[*h]
}
