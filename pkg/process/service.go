package process

// Service represents a service that can be started and stopped.
type Service interface {
	// Start starts the service or reports an error.
	Start() error

	// Stop stops the service.
	Stop()
}
