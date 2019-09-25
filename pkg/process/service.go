// Package process facilitates connecting all the components of the Aleph protocol defined in other packages in a program that executes the protocol.
// Most of the works happens in subpackages, this one only defines configuration and a service interface.
package process

// Service represents a service that can be started and stopped.
type Service interface {
	// Start the service or report an error.
	Start() error

	// Stop the service.
	Stop()
}
