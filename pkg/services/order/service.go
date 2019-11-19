// Package order implements a service for computing the linear order of units.
package order

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

type service struct {
	pid            uint16
	linearOrdering gomel.LinearOrdering
	orderedUnits   chan<- []gomel.Unit
	primeAlert     <-chan struct{}
	exitChan       chan struct{}
	wg             sync.WaitGroup
	log            zerolog.Logger
}

// NewService constructs an ordering service.
// This service sorts units in linear order.
// orderedUnits is an output channel where it writes these units in order.
// Ordering is attempted when the returned function is called on a prime unit.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, conf *config.Order, orderedUnits chan<- []gomel.Unit, log zerolog.Logger) gomel.Service {
	primeAlert := make(chan struct{}, 1)
	s := &service{
		pid:            conf.Pid,
		linearOrdering: linear.NewOrdering(dag, randomSource, conf.OrderStartLevel, conf.CRPFixedPrefix, log),
		orderedUnits:   orderedUnits,
		primeAlert:     primeAlert,
		exitChan:       make(chan struct{}),
		log:            log,
	}

	alertIfPrime := func(u gomel.Unit) {
		if gomel.Prime(u) {
			select {
			case primeAlert <- struct{}{}:
			default:
			}
		}
	}
	dag.AfterInsert(alertIfPrime)

	return s
}

func (s *service) attemptOrdering() {
	defer s.wg.Done()
	for {
		select {
		case <-s.primeAlert:
			for s.linearOrdering.DecideTiming() != nil {
				s.extendOrder()
			}
		case <-s.exitChan:
			return
		}
	}
}

func (s *service) extendOrder() {
	units := s.linearOrdering.TimingRound()
	s.orderedUnits <- units
	for _, u := range units {
		if u.Creator() == s.pid {
			s.log.Info().Int(logging.Height, u.Height()).Msg(logging.OwnUnitOrdered)
		}
	}
	s.log.Info().Int(logging.Size, len(units)).Msg(logging.LinearOrderExtended)
}

func (s *service) Start() error {
	s.wg.Add(1)
	go s.attemptOrdering()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	close(s.exitChan)
	s.wg.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}
