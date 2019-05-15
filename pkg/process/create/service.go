package create

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	creator *adjustingCreator
}

func makeFinal(maxLevel, maxHeight int, finished chan<- struct{}) func(gomel.Unit) bool {
	return func(created gomel.Unit) bool {
		if created.Level() >= maxLevel || created.Height() >= maxHeight {
			close(finished)
			return true
		}
		return false
	}
}

// NewService creates a new creating service for the given poset, with the given configuration.
// The service will close done when it stops.
func NewService(poset gomel.Poset, config *process.Create, done chan<- struct{}) (process.Service, error) {
	return &service{
		creator: newAdjustingCreator(poset, config.ID, config.MaxParents, config.PrivateKey, config.InitialDelay, makeFinal(config.MaxLevel, config.MaxHeight, done)),
	}, nil
}

func (s *service) Start() error {
	s.creator.start()
	return nil
}

func (s *service) Stop() {
	s.creator.stop()
}
