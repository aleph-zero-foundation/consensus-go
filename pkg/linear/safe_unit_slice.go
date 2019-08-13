package linear

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type safeUnitSlice struct {
	sync.RWMutex
	contents []gomel.Unit
}

func newSafeUnitSlice(nEmpty int) *safeUnitSlice {
	contents := make([]gomel.Unit, nEmpty)
	return &safeUnitSlice{
		contents: contents,
	}
}

func (s *safeUnitSlice) appendOrIgnore(level int, u gomel.Unit) {
	s.Lock()
	defer s.Unlock()
	if len(s.contents) == level {
		s.contents = append(s.contents, u)
	}
}

func (s *safeUnitSlice) length() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.contents)
}

func (s *safeUnitSlice) get(pos int) gomel.Unit {
	s.RLock()
	defer s.RUnlock()
	return s.contents[pos]
}
