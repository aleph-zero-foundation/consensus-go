package sync

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// In represents a protocol for incomming synchronization
type In interface {
	Run(gomel.Poset, network.Connection)
	OnDone(func())
}

// Out represents a protocol for outgoing synchronization
type Out interface {
	Run(gomel.Poset, network.Connection)
	OnDone(func())
}
