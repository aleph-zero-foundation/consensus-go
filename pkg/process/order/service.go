package order

import (
	"github.com/rs/zerolog"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/linear"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

// Order service is sorting units in linear order
// - attemptTimingRequests is an external channel from which we read that we should run attemptTimingDecision
// - orderedUnits is an output channel where we write units in linear order
// - extendOrderRequests is a channel used by the go routine which is choosing timing units to inform
//   the go routine which is ordering units that a new timingUnit has been chosen
// - currentRound is the round up to which we have chosen timing units
type service struct {
	linearOrdering        gomel.LinearOrdering
	attemptTimingRequests <-chan int
	extendOrderRequests   chan int
	orderedUnits          chan<- gomel.Unit
	currentRound          int
	exitChan              chan struct{}
	wg                    sync.WaitGroup
	log                   zerolog.Logger
}

// NewService is a constructor of an ordering service
func NewService(poset gomel.Poset, config *process.Order, attemptTimingRequests <-chan int, orderedUnits chan<- gomel.Unit, log zerolog.Logger) (process.Service, error) {
	return &service{
		linearOrdering:        linear.NewOrdering(poset, config.VotingLevel, config.PiDeltaLevel),
		attemptTimingRequests: attemptTimingRequests,
		orderedUnits:          orderedUnits,
		extendOrderRequests:   make(chan int, 10),
		exitChan:              make(chan struct{}),
		currentRound:          0,
		log:                   log,
	}, nil
}

func (s *service) attemptOrdering() {
	for {
		select {
		case highest := <-s.attemptTimingRequests: // level of the most recent prime unit
			for s.linearOrdering.DecideTimingOnLevel(s.currentRound) != nil {
				s.log.Info().Int("h", highest).Int("r", s.currentRound).Msg(logging.NewTimingUnit)
				s.extendOrderRequests <- s.currentRound
				s.currentRound++
			}
		case <-s.exitChan:
			close(s.extendOrderRequests)
			s.wg.Done()
			return
		}
	}
}

func (s *service) extendOrder() {
	for round := range s.extendOrderRequests {
		units := s.linearOrdering.TimingRound(round)
		for _, u := range units {
			s.orderedUnits <- u
		}
		s.log.Info().Int("n", len(units)).Msg(logging.LinearOrderExtended)
	}
	close(s.orderedUnits)
	s.wg.Done()
}

// Start is a function which starts the service
func (s *service) Start() error {
	s.wg.Add(2)
	go s.attemptOrdering()
	go s.extendOrder()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

// Stop is the function that stops the service
func (s *service) Stop() {
	close(s.exitChan)
	s.wg.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}
