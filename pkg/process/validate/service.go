package validate

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	validator  *validator
	unitSource <-chan gomel.Unit
	exitChan   chan struct{}
}

// NewService creates a new transaction validation service for the given poset, with the given configuration.
func NewService(poset gomel.Poset, config *process.Validate, unitSource <-chan gomel.Unit) (process.Service, error) {
	return &service{
		unitSource: unitSource,
		exitChan:   make(chan struct{}),
		validator:  newValidator(),
	}, nil
}

func (s *service) main() {
	for {
		select {
		case u := <-s.unitSource:
			for _, t := range u.Txs() {
				s.validator.validate(t)
			}
		case <-s.exitChan:
			return
		}
	}
}
func (s *service) Start() error {
	go s.main()
	return nil
}

func (s *service) Stop() {
	close(s.exitChan)
}
