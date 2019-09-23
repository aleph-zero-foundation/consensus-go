// Package sync defines the primitives needed for various synchronization algorithms.
package sync

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

// Server is responsible for handling a sync protocol.
type Server interface {
	// Start the server.
	Start()
	// StopIn stops handling incoming synchronizations.
	StopIn()
	// StopOut stops handling outgoing synchronizations.
	StopOut()
	// SetFallback registers a Fallback that will be used to query information about problematic preunits.
	SetFallback(Fallback)
}

// MulticastServer is a Server that can multicast units to other committee members.
type MulticastServer interface {
	Server
	// Send multicasts a unit to all other committee members.
	Send(gomel.Unit)
}
