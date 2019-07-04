package network

import "time"

// Listener waits for incoming connections
type Listener interface {
	Listen(time.Duration) (Connection, error)
}
