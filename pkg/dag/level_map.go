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

func (lm *fiberMap) getFiber(value int) (gomel.SlottedUnits, error) {
	lm.mx.RLock()
	defer lm.mx.RUnlock()
	result, ok := lm.content[value]
	if !ok {
		return nil, newNoSuchValueError(value)
	}
	return result, nil
}

func (lm *fiberMap) Len() int {
	lm.mx.RLock()
	defer lm.mx.RUnlock()
	return lm.length
}

func (lm *fiberMap) extendBy(nValues int) {
	lm.mx.Lock()
	defer lm.mx.Unlock()
	for i := lm.length; i < lm.length+nValues; i++ {
		lm.content[i] = newSlottedUnits(lm.width)
	}
	lm.length += nValues
}
