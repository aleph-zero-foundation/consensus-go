package network

// ConnectionServer is a central for both handling incomming requests for connections and
// establishing outgoing connections
type ConnectionServer interface {

	// Listen waits for requests, manages them, and after succesful setup sends a ready connection to a channel that is returned
	Listen() (chan Connection, error)

	// Dial starts a service that periodically tries to establish a new connection to
	// a remote peer
	Dial() chan Connection

	// Stop halts both Listen and Dial services.
	Stop()
}
