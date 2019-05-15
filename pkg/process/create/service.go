package create

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
}

// NewService creates a new creating service for the given poset, with the given configuration.
// The service will close done when it stops.
func NewService(poset gomel.Poset, config *process.Create, done chan<- struct{}) (process.Service, error) {
	// TODO: implement.
	return &service{}, nil
}

func (s *service) Start() error {
	// TODO: implement.
	return nil
}

func (s *service) Stop() {
	// TODO: implement.
}
