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
	contents := []gomel.Unit{}
	for i := 0; i < nEmpty; i++ {
		contents = append(contents, nil)
	}
	return &safeUnitSlice{
		contents: contents,
	}
}

func (s *safeUnitSlice) pushBack(u gomel.Unit) {
	s.Lock()
	defer s.Unlock()
	s.contents = append(s.contents, u)
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
