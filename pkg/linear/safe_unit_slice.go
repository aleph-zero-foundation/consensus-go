package linear

import (
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type safeUnitSlice struct {
	sync.RWMutex
	contents []gomel.Unit
}

func newSafeUnitSlice() *safeUnitSlice {
	return &safeUnitSlice{
		contents: []gomel.Unit{},
	}
}

func (s *safeUnitSlice) safeAppend(u gomel.Unit) {
	s.Lock()
	defer s.Unlock()
	s.contents = append(s.contents, u)
}

func (s *safeUnitSlice) safeLen() int {
	s.Lock()
	defer s.Unlock()
	return len(s.contents)
}

func (s *safeUnitSlice) safeGet(pos int) gomel.Unit {
	s.Lock()
	defer s.Unlock()
	return s.contents[pos]
}
