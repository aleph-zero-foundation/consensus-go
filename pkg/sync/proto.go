package sync

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Protocol represents a protocol for incoming/outgoing synchronization
type Protocol interface {
	Run(gomel.Poset, Connection)
}
