package sync

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

// Fallback describes what should be done when encountering a unit with unknown parents.
type Fallback interface {
	// Run takes the unit with unknown parents and falls back appropriately.
	Run(gomel.Preunit)
}

type noop struct{}

func (f noop) Run(gomel.Preunit) {}

// NopFallback is a fallback that does nothing
func NopFallback() Fallback {
	return noop{}
}
