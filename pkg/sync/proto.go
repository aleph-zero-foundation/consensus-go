package sync

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// Protocol represents a protocol for incoming/outgoing synchronization
type Protocol interface {
	Run(gomel.Poset, network.Connection)
}
