package order

import (
	"sync"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
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
	pid                 int
	linearOrdering      gomel.LinearOrdering
	extendOrderRequests chan int
	orderedUnits        chan<- []gomel.Unit
	currentRound        int
	exitChan            chan struct{}
	wg                  sync.WaitGroup
	log                 zerolog.Logger
}

// NewService is a constructor of an ordering service
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Order, orderedUnits chan<- []gomel.Unit, log zerolog.Logger) (process.Service, gomel.Callback, error) {
	return &service{
		pid:                 config.Pid,
		linearOrdering:      linear.NewOrdering(dag, randomSource, config.VotingLevel, config.PiDeltaLevel),
		orderedUnits:        orderedUnits,
		extendOrderRequests: make(chan int, 10),
		exitChan:            make(chan struct{}),
		currentRound:        config.OrderStartLevel,
		log:                 log,
	}, func(gomel.Preunit, gomel.Unit, error) {}, nil
}

func (s *service) attemptOrdering() {
	defer close(s.extendOrderRequests)
	defer s.wg.Done()
	for {
		//select {
		//case highest, ok := <-s.attemptTimingRequests: // level of the most recent prime unit
		//	if !ok {
		//		<-s.exitChan
		//		return
		//	}
		//	for s.linearOrdering.DecideTimingOnLevel(s.currentRound) != nil {
		//		s.log.Info().Int(logging.Height, highest).Int(logging.Round, s.currentRound).Msg(logging.NewTimingUnit)
		//		s.extendOrderRequests <- s.currentRound
		//		s.currentRound++
		//	}
		//case <-s.exitChan:
		//	return
		//}
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
