package random

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// SyncCSMap is a threadsafe implementation of the map
// unit's hash => coinShare included in the unit.
type SyncCSMap struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.CoinShare
}

// NewSyncCSMap returns new SyncCSMap object
func NewSyncCSMap() *SyncCSMap {
	return &SyncCSMap{contents: make(map[gomel.Hash]*tcoin.CoinShare)}
}

// Add adds a coinshare to the map
func (sm *SyncCSMap) Add(h *gomel.Hash, elem *tcoin.CoinShare) {
	sm.Lock()
	defer sm.Unlock()
	sm.contents[*h] = elem
}

// Get returns a coinshare saved in the unit having given hash
func (sm *SyncCSMap) Get(h *gomel.Hash) *tcoin.CoinShare {
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
