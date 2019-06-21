package network

// ConnectionServer handles incoming requests for connections
type ConnectionServer interface {
	// Start waits for requests and manages them
	Start() error

	// Stop halts the connection server.
	Stop()
}
