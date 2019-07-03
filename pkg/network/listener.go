package network

// Listener waits for incoming connections
type Listener interface {
	Listen() (Connection, error)
}
