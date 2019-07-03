package sync

// Protocol represents a protocol for incoming/outgoing synchronization.
type Protocol interface {
	In()
	Out()
}
