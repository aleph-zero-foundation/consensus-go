package dag

import (
	"fmt"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type fiberMap struct {
	content map[int]gomel.SlottedUnits
	width   uint16
	length  int
	mx      sync.RWMutex
}

type noSuchValueError struct {
	value int
}

func newNoSuchValueError(value int) *noSuchValueError {
	return &noSuchValueError{value}
}

func (e *noSuchValueError) Error() string {
	return fmt.Sprintf("value %v does not exist", e.value)
}

func newFiberMap(width uint16, initialLen int) *fiberMap {
	newMap := &fiberMap{
		content: make(map[int]gomel.SlottedUnits),
		width:   width,
		length:  initialLen,
		mx:      sync.RWMutex{},
	}
	for i := 0; i < initialLen; i++ {
		newMap.content[i] = newSlottedUnits(width)
	}
	return newMap
}

func (fm *fiberMap) getFiber(value int) (gomel.SlottedUnits, error) {
	fm.mx.RLock()
	defer fm.mx.RUnlock()
	result, ok := fm.content[value]
	if !ok {
		return nil, newNoSuchValueError(value)
	}
	return result, nil
}

func (fm *fiberMap) Len() int {
	fm.mx.RLock()
	defer fm.mx.RUnlock()
	return fm.length
}

func (fm *fiberMap) extendBy(nValues int) {
	fm.mx.Lock()
	defer fm.mx.Unlock()
	for i := fm.length; i < fm.length+nValues; i++ {
		fm.content[i] = newSlottedUnits(fm.width)
	}
	fm.length += nValues
}

func (fm *fiberMap) get(heights []int) [][]gomel.Unit {
	fm.mx.RLock()
	defer fm.mx.RUnlock()
	nProc := len(heights)
	result := make([][]gomel.Unit, nProc)
	for pid, h := range heights {
		if h == -1 {
			continue
		}
		if su, ok := fm.content[h]; ok {
			result[pid] = su.Get(uint16(pid))
		}
	}
	return result
}
