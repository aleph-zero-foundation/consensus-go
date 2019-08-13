package network

import "time"

// Server establishes network connections.
type Server interface {
	// Dial connects to a committee member identified by pid and returns the resulting connection or an error.
	Dial(pid uint16) (Connection, error)
	// Listen listens for incoming connection for the given time and returns it if successful, or times out.
	Listen(time.Duration) (Connection, error)
}
