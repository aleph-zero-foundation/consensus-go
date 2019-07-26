package sync

// Server is responsible for handling a sync protocol.
type Server interface {
	// Start starts server
	Start()
	// StopIn stops handling incoming synchronizations
	StopIn()
	// StopOut stops handling outgoing synchronizations
	StopOut()
}
