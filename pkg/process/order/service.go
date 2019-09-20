// Package order implements a service for computing the linear order of units.
package order

import (
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

type service struct {
	pid                 uint16
	linearOrdering      gomel.LinearOrdering
	extendOrderRequests chan int
	orderedUnits        chan<- []gomel.Unit
	currentRound        int
	primeAlert          <-chan struct{}
	exitChan            chan struct{}
	wg                  sync.WaitGroup
	log                 zerolog.Logger
}

// NewService constructs an ordering service.
// This service sorts units in linear order.
// orderedUnits is an output channel where it writes these units in order.
// Ordering is attempted when a prime unit is added to the returned dag.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Order, orderedUnits chan<- []gomel.Unit, log zerolog.Logger) (process.Service, func(gomel.Unit)) {
	primeAlert := make(chan struct{}, 1)
	return &service{
			pid:                 config.Pid,
			linearOrdering:      linear.NewOrdering(dag, randomSource, config.OrderStartLevel, config.CRPFixedPrefix, log),
			orderedUnits:        orderedUnits,
			extendOrderRequests: make(chan int, 10),
			primeAlert:          primeAlert,
			exitChan:            make(chan struct{}),
			currentRound:        config.OrderStartLevel,
			log:                 log,
		}, func(u gomel.Unit) {
			if gomel.Prime(u) {
				select {
				case primeAlert <- struct{}{}:
				default:
				}
			}
		}
}

func (s *service) attemptOrdering() {
	defer close(s.extendOrderRequests)
	defer s.wg.Done()
	for {
		select {
		case <-s.primeAlert:
			for s.linearOrdering.DecideTiming() != nil {
				s.extendOrderRequests <- s.currentRound
				s.currentRound++
			}
		case <-s.exitChan:
			return
		}
	}
}

func (s *service) extendOrder() {
	for round := range s.extendOrderRequests {
		units := s.linearOrdering.TimingRound(round)
		s.orderedUnits <- units
		for _, u := range units {
			if u.Creator() == s.pid {
				s.log.Info().Int(logging.Height, u.Height()).Msg(logging.OwnUnitOrdered)
			}
		}
		s.log.Info().Int(logging.Size, len(units)).Msg(logging.LinearOrderExtended)
	}
	close(s.orderedUnits)
	s.wg.Done()
}

func (s *service) Start() error {
	s.wg.Add(2)
	go s.attemptOrdering()
	go s.extendOrder()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	close(s.exitChan)
	s.wg.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}
