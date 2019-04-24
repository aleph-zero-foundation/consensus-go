package network

// Connection represents a connection between two processes.
type Connection interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}

// Connector opens connections between processes
type Connector interface {
	Connect(int, int) Connection
}
