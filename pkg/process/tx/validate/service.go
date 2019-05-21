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
func NewService(poset gomel.Poset, config *process.TxValidate, unitSource <-chan gomel.Unit) (process.Service, error) {
	validator, err := newValidator(config.UserDb)
	if err != nil {
		return nil, err
	}
	return &service{
		unitSource: unitSource,
		exitChan:   make(chan struct{}),
		validator:  validator,
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
