package growing

import (
	"fmt"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type levelMap struct {
	content map[int]gomel.SlottedUnits
	width   int
	height  int
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

func newLevelMap(width, initialHeight int) *levelMap {
	newMap := &levelMap{
		content: make(map[int]gomel.SlottedUnits),
		width:   width,
		height:  initialHeight,
		mx:      sync.RWMutex{},
	}
	for i := 0; i < initialHeight; i++ {
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

func (lm *levelMap) getHeight() int {
	lm.mx.RLock()
	defer lm.mx.RUnlock()
	return lm.height
}

func (lm *levelMap) extendBy(nLevels int) {
	lm.mx.Lock()
	defer lm.mx.Unlock()
	for i := lm.height; i < lm.height+nLevels; i++ {
		lm.content[i] = newSlottedUnits(lm.width)
	}
	lm.height += nLevels
}
