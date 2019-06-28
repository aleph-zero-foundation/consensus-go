package network

// Multicaster allows to send out messages to multiple recipients
type Multicaster interface {
	Write([]byte) (int, error)
	Flush() error
	Close() error
}
