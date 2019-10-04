package gomel

// Service represents a service that can be started and stopped.
type Service interface {
	// Start the service or report an error.
	Start() error

	// Stop the service.
	Stop()
}
