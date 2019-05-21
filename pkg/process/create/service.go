package create

import (
	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	creator *adjustingCreator
	log     zerolog.Logger
}

// makeFinal binds some constants into a function that should be called after adding a unit to the poset.
// The resulting function returns true if the added unit is determined to be the final one to be created.
func makeFinal(maxLevel, maxHeight int, finished chan<- struct{}, primeUnitCreated chan<- int) func(gomel.Unit) bool {
	return func(created gomel.Unit) bool {
		if gomel.Prime(created) {
			primeUnitCreated <- created.Level()
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
func NewService(poset gomel.Poset, config *process.Create, posetFinished chan<- struct{}, primeUnitCreated chan<- int, txSource <-chan *gomel.Tx, log zerolog.Logger) (process.Service, error) {
	return &service{
		creator: newAdjustingCreator(poset, config.Pid, config.MaxParents, config.PrivateKey, config.InitialDelay, config.AdjustFactor, makeFinal(config.MaxLevel, config.MaxHeight, posetFinished, primeUnitCreated), config.Txpu, txSource),
		log:     log,
	}, nil
}

func (s *service) Start() error {
	s.log.Info().Msg(logging.ServiceStarted)
	s.creator.start()
	return nil
}

func (s *service) Stop() {
	s.creator.stop()
	s.log.Info().Msg(logging.ServiceStopped)
}
