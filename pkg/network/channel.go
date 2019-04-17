package network

// Channel represents a connection between two processes.
type Channel interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}
