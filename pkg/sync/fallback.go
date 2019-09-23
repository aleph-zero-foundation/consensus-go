package sync

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

// Fallback can find out information about an unknown preunit.
type Fallback interface {
	// FindOut requests information about a problematic preunit.
	FindOut(gomel.Preunit)
}
