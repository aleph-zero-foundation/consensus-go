package validate

import (
	"sync"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	unitSource <-chan gomel.Unit
	exitChan   chan struct{}
	log        zerolog.Logger
	wg         sync.WaitGroup
}

// NewService creates a new transaction validation service for the given dag, with the given configuration.
func NewService(dag gomel.Dag, config *process.TxValidate, unitSource <-chan gomel.Unit, log zerolog.Logger) (process.Service, error) {
	return &service{
		unitSource: unitSource,
		exitChan:   make(chan struct{}),
		log:        log,
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
			s.log.Info().Int(logging.Size, len(u.Data())).Msg(logging.DataValidated)
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
