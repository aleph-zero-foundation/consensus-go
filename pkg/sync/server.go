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
}

// QueryServer is a Server that can find out information about an unknown preunit.
type QueryServer interface {
	Server
	FindOut(gomel.Preunit)
}

// MulticastServer is a Server that can multicast units to other committee members.
type MulticastServer interface {
	Server
	Send(gomel.Unit)
}
