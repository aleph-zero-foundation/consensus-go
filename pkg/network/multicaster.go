package network

// Connection represents a connection between two processes.
type Multicaster interface {
	Write([]byte) (int, error)
	Flush() error
	Close() error
}
