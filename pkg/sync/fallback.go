package sync

import gomel "gitlab.com/alephledger/consensus-go/pkg"

// Fallback describes what should be done when encountering a unit with unknown parents.
type Fallback interface {
	// Run takes the unit with unknown parents and falls back appropriately.
	Run(gomel.Preunit)
}

type noop struct{}

func (f noop) Run(gomel.Preunit) {}

// Noop is a fallback that does nothing
func Noop() Fallback {
	return noop{}
}
