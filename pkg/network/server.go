package network

// ConnectionServer is a central for both handling incomming requests for connections and
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

	// Dial starts a service that periodically tries to establish a new connection to
	// a remote peer
	Dial()

	// Stop halts both Listen and Dial services.
	Stop()
}
