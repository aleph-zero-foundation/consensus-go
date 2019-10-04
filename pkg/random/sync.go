// Package random defines data structures used by various random sources.
//
// The actual random sources are implemented in subpackages.
package random

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// SyncCSMap is a thread-safe implementation of the map
// unit's hash => coinShare included in the unit.
type SyncCSMap struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.CoinShare
}

// NewSyncCSMap returns new SyncCSMap object.
func NewSyncCSMap() *SyncCSMap {
	return &SyncCSMap{contents: make(map[gomel.Hash]*tcoin.CoinShare)}
}

// Add adds a coinshare to the map.
func (sm *SyncCSMap) Add(h *gomel.Hash, elem *tcoin.CoinShare) {
	sm.Lock()
	defer sm.Unlock()
	sm.contents[*h] = elem
}

// Get returns a coinshare saved in the unit of the given hash.
func (sm *SyncCSMap) Get(h *gomel.Hash) *tcoin.CoinShare {
	sm.RLock()
	defer sm.RUnlock()
	return sm.contents[*h]
}

// SyncTCMap is a thread-safe implementation of the map
// unit's hash => thresholdCoin contained in it.
type SyncTCMap struct {
	sync.RWMutex
	contents map[gomel.Hash]*tcoin.ThresholdCoin
}

// NewSyncTCMap return new SyncTCMap object.
func NewSyncTCMap() *SyncTCMap {
	return &SyncTCMap{contents: make(map[gomel.Hash]*tcoin.ThresholdCoin)}
}

// Add adds a tcoin to the map.
func (sm *SyncTCMap) Add(h *gomel.Hash, elem *tcoin.ThresholdCoin) {
	sm.Lock()
	defer sm.Unlock()
	sm.contents[*h] = elem
}

// Get returns a tcoin saved in the unit of the given hash.
func (sm *SyncTCMap) Get(h *gomel.Hash) *tcoin.ThresholdCoin {
	sm.RLock()
	defer sm.RUnlock()
	return sm.contents[*h]
}

// SyncBytesSlice is a thread-safe implementation of a slice of bytes slices.
type SyncBytesSlice struct {
	sync.RWMutex
	contents [][]byte
}

// NewSyncBytesSlice returns an empty SyncBytesSlice.
func NewSyncBytesSlice() *SyncBytesSlice {
	return &SyncBytesSlice{
		contents: [][]byte{},
	}
}

// AppendOrIgnore appends the given data at the end of the slice if the current
// length of the slice is equal to the given length, otherwise it does nothing.
func (s *SyncBytesSlice) AppendOrIgnore(length int, data []byte) {
	s.Lock()
	defer s.Unlock()
	if len(s.contents) == length {
		s.contents = append(s.contents, data)
	}
}

// Length returns the number of elements in the slice.
func (s *SyncBytesSlice) Length() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.contents)
}

// Get returns an element of the slice.
func (s *SyncBytesSlice) Get(pos int) []byte {
	s.RLock()
	defer s.RUnlock()
	if pos < len(s.contents) {
		return s.contents[pos]
	}
	return nil
}
