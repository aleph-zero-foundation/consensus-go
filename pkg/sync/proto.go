package sync

import "gitlab.com/alephledger/consensus-go/pkg/network"

// Protocol represents a protocol for incoming/outgoing synchronization.
type Protocol interface {
	In(network.Connection)
	Out()
}
