package gomel

// Service represents a functionality that can be started and stopped.
type Service interface {
	Start()
	Stop()
}
