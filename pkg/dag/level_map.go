package dag

import (
	"fmt"
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type levelMap struct {
	content map[int]gomel.SlottedUnits
	width   uint16
	length  int
	mx      sync.RWMutex
}

type noSuchLevelError struct {
	level int
}

func newNoSuchLevelError(level int) *noSuchLevelError {
	return &noSuchLevelError{level}
}

func (e *noSuchLevelError) Error() string {
	return fmt.Sprintf("Level %v does not exist.", e.level)
}

func newLevelMap(width uint16, initialLen int) *levelMap {
	newMap := &levelMap{
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

func (lm *levelMap) getLevel(level int) (gomel.SlottedUnits, error) {
	lm.mx.RLock()
	defer lm.mx.RUnlock()
	result, ok := lm.content[level]
	if !ok {
		return nil, newNoSuchLevelError(level)
	}
	return result, nil
}

func (lm *levelMap) Len() int {
	lm.mx.RLock()
	defer lm.mx.RUnlock()
	return lm.length
}

func (lm *levelMap) extendBy(nLevels int) {
	lm.mx.Lock()
	defer lm.mx.Unlock()
	for i := lm.length; i < lm.length+nLevels; i++ {
		lm.content[i] = newSlottedUnits(lm.width)
	}
	lm.length += nLevels
}
