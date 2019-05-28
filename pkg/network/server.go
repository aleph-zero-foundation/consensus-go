package network

// ConnectionServer is a central for both handling incoming requests for connections and
// establishing outgoing connections
type ConnectionServer interface {

	// ListenChannel returns the channel into which incoming established connections
	// will be pushed
	ListenChannel() <-chan Connection

	// DialChannel returns the channel into which outgoing established connections
	// will be pushed
	DialChannel() <-chan Connection

	// Listen waits for requests and manages them
	Listen() error

	// StartDialing starts a service that periodically tries to establish a new
	// connection to a remote peer
	StartDialing()

	// Stop halts both Listen and Dialing services.
	Stop()
}
