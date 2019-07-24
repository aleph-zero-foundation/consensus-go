package gomel

import (
	"fmt"
	"strings"
	"sync"
)

// ErrGroup spawns multiple goroutines, waits until they finish, and returns a concatenated error
type ErrGroup struct {
	wg      sync.WaitGroup
	errFlag int64
	errors  []string
}

// NewErrGroup creates a new instance of ErrGroup
func NewErrGroup() *ErrGroup {
	return &ErrGroup{}
}

// Go runs tasks in goroutines, gathers potential errors, and returns a concatenation of them
func (eg *ErrGroup) Go(tasks []func() error) error {
	eg.errors = make([]string, len(tasks))
	eg.wg.Add(len(tasks))
	for i, task := range tasks {
		go func(i int, task func() error) {
			defer eg.wg.Done()
			if err := task(); err != nil {
				eg.errFlag = 1
				eg.errors[i] = err.Error()
			}
		}(i, task)
	}

	eg.wg.Wait()
	if eg.errFlag == 1 {
		return fmt.Errorf(strings.Join(eg.errors, "\n"))
	}

	return nil
}
