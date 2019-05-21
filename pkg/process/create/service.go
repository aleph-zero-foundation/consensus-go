package create

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	creator *adjustingCreator
}

// makeFinal binds some constants into a function that should be called after adding a unit to the poset.
// The resulting function returns true if the added unit is determined to be the final one to be created.
func makeFinal(maxLevel, maxHeight int, finished chan<- struct{}, primeUnitCreated chan<- struct{}) func(gomel.Unit) bool {
	return func(created gomel.Unit) bool {
		if gomel.Prime(created) {
			primeUnitCreated <- struct{}{}
		}
		if created.Level() >= maxLevel || created.Height() >= maxHeight {
			close(finished)
			return true
		}
		return false
	}
}

// NewService creates a new creating service for the given poset, with the given configuration.
// The service will close posetFinished when it stops.
func NewService(poset gomel.Poset, config *process.Create, posetFinished chan<- struct{}, primeUnitCreated chan<- struct{}, txSource <-chan *gomel.Tx) (process.Service, error) {
	return &service{
		creator: newAdjustingCreator(poset, config.Pid, config.MaxParents, config.PrivateKey, config.InitialDelay, config.AdjustFactor, makeFinal(config.MaxLevel, config.MaxHeight, posetFinished, primeUnitCreated), config.Txpu, txSource),
	}, nil
}

func (s *service) Start() error {
	s.creator.start()
	return nil
}

func (s *service) Stop() {
	s.creator.stop()
}
