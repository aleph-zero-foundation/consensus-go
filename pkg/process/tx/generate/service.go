// Package generate implements a service simulating incoming transactions.
//
// The service actually generates pseudorandom strings of bytes of length roughly corresponding to
// the expected length of a number of transactions.
// Since it exists mostly for testing purposes it will probably soon be deprecated.
package generate

import (
	"math/rand"
	"sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	txpu     int
	txChan   chan<- []byte
	exitChan chan struct{}
	log      zerolog.Logger
	wg       sync.WaitGroup
}

// NewService creates a service generating transactions.
func NewService(dag gomel.Dag, config *process.TxGenerate, txChan chan<- []byte, log zerolog.Logger) (process.Service, error) {
	return &service{
		txpu:     config.Txpu,
		txChan:   txChan,
		exitChan: make(chan struct{}),
		log:      log,
	}, nil
}

func (s *service) generateRandom() []byte {
	txpu := s.txpu
	size := 15*txpu + rand.Intn(txpu)
	result := make([]byte, size)
	rand.Read(result)
	return result
}

func (s *service) main() {
	for {
		data := s.generateRandom()
		select {
		case s.txChan <- data:
		case <-s.exitChan:
			close(s.txChan)
			s.wg.Done()
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
