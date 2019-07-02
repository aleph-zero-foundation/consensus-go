package random

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
)

type safeCSMap struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.CoinShare
}

func newSafeCSMap() *safeCSMap {
	return &safeCSMap{contents: make(map[gomel.Hash]*tcoin.CoinShare)}
}

func (sm *safeCSMap) remove(h *gomel.Hash) {
	sm.Lock()
	defer sm.Unlock()
	delete(sm.contents, *h)
}

func (sm *safeCSMap) add(h *gomel.Hash, elem *tcoin.CoinShare) {
	sm.Lock()
	defer sm.Unlock()
	sm.contents[*h] = elem
}

func (sm *safeCSMap) get(h *gomel.Hash) *tcoin.CoinShare {
	sm.RLock()
	defer sm.RUnlock()
	return sm.contents[*h]
}

type safeTCMap struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.ThresholdCoin
}

func newSafeTCMap() *safeTCMap {
	return &safeTCMap{contents: make(map[gomel.Hash]*tcoin.ThresholdCoin)}
}

func (sm *safeTCMap) remove(h *gomel.Hash) {
	sm.Lock()
	defer sm.Unlock()
	delete(sm.contents, *h)
}

func (sm *safeTCMap) add(h *gomel.Hash, elem *tcoin.ThresholdCoin) {
	sm.Lock()
	defer sm.Unlock()
	sm.contents[*h] = elem
}

func (sm *safeTCMap) get(h *gomel.Hash) *tcoin.ThresholdCoin {
	sm.RLock()
	defer sm.RUnlock()
	return sm.contents[*h]
}
