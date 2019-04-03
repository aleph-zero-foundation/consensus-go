package growing

import (
    "sync"
    "fmt"
    gomel "gitlab.com/alephledger/consensus-go/pkg"
)


type levelMap struct {
    content  map[int]gomel.SlottedUnits
    width    int
    height   int
    mx       sync.RWMutex
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


func newLevelMap(initial_levels, n int) *levelMap {
    newMap := &levelMap{
        content: make(map[int]gomel.SlottedUnits),
        width:   n,
        height:  initial_levels,
        mx:      sync.RWMutex{},
    }
    for i := 0; i < initial_levels; i++ {
        newMap.content[i] = newSlottedUnits(n)
    }
    return newMap
}

func (lm *levelMap) getLevel(level int) (gomel.SlottedUnits, error) {
    lm.mx.RLock()
    defer lm.mx.RUnlock()
    result, ok := lm.content[level]
    if ok {
        return result, nil
    } else {
        return nil, newNoSuchLevelError(level)
    }
}

func (lm *levelMap) getHeight() int {
    lm.mx.RLock()
    defer lm.mx.RUnlock()
    return lm.height
}

func (lm *levelMap) extendBy(nLevels int) {
    lm.mx.Lock()
    defer lm.mx.Unlock()
    for i := lm.height; i < lm.height + nLevels; i++ {
        lm.content[i] = newSlottedUnits(lm.width)
    }
    lm.height += nLevels
}
