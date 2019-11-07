// Package sync defines the primitives needed for various synchronization algorithms.
package sync

// Server is responsible for handling a sync protocol.
type Server interface {
	// Start the server.
	Start()
	// StopIn stops handling incoming synchronizations.
	StopIn()
	// StopOut stops handling outgoing synchronizations.
	StopOut()
}
