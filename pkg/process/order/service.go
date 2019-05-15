package order

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
}

// NewService creates a new ordering service for the given poset, with the given configuration.
func NewService(poset gomel.Poset, config *process.Order) (process.Service, error) {
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
