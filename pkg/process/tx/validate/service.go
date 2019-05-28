package validate

import (
	"sync"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/transactions"
)

type service struct {
	validator  *validator
	unitSource <-chan gomel.Unit
	exitChan   chan struct{}
	log        zerolog.Logger
	wg         sync.WaitGroup
}

// NewService creates a new transaction validation service for the given poset, with the given configuration.
func NewService(poset gomel.Poset, config *process.TxValidate, unitSource <-chan gomel.Unit, log zerolog.Logger) (process.Service, error) {
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
	defer s.wg.Done()
	for {
		select {
		case u, ok := <-s.unitSource:
			if !ok {
				<-s.exitChan
				return
			}
			txsEncoded, cErr := transactions.Decompress(u.Data())
			txs, dErr := transactions.Decode(txsEncoded)
			if cErr != nil && dErr != nil {
				for _, t := range txs {
					s.validator.validate(t)
				}
			}
		case <-s.exitChan:
			return
		}
	}
}

func (s *service) Start() error {
	s.wg.Add(1)
	go s.main()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	close(s.exitChan)
	s.wg.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}
