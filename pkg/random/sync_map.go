package random

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

type syncCSMap struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.CoinShare
}

func newSyncCSMap() *syncCSMap {
	return &syncCSMap{contents: make(map[gomel.Hash]*tcoin.CoinShare)}
}

func (sm *syncCSMap) remove(h *gomel.Hash) {
	sm.Lock()
	defer sm.Unlock()
	delete(sm.contents, *h)
}

func (sm *syncCSMap) add(h *gomel.Hash, elem *tcoin.CoinShare) {
	sm.Lock()
	defer sm.Unlock()
	sm.contents[*h] = elem
}

func (sm *syncCSMap) get(h *gomel.Hash) *tcoin.CoinShare {
	sm.RLock()
	defer sm.RUnlock()
	return sm.contents[*h]
}

type syncTCMap struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.ThresholdCoin
}

func newSyncTCMap() *syncTCMap {
	return &syncTCMap{contents: make(map[gomel.Hash]*tcoin.ThresholdCoin)}
}

func (sm *syncTCMap) remove(h *gomel.Hash) {
	sm.Lock()
	defer sm.Unlock()
	delete(sm.contents, *h)
}

func (sm *syncTCMap) add(h *gomel.Hash, elem *tcoin.ThresholdCoin) {
	sm.Lock()
	defer sm.Unlock()
	sm.contents[*h] = elem
}

func (sm *syncTCMap) get(h *gomel.Hash) *tcoin.ThresholdCoin {
	sm.RLock()
	defer sm.RUnlock()
	return sm.contents[*h]
}
