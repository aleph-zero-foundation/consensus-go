// Package create implements a service for creating new units.
package create

import (
	"sync"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

const (
	positiveJerk = 1.01
	negativeJerk = 0.90
)

type service struct {
	dag              gomel.Dag
	adder            gomel.Adder
	randomSource     gomel.RandomSource
	pid              uint16
	maxParents       uint16
	primeOnly        bool
	canSkipLevel     bool
	maxLevel         int
	privKey          gomel.PrivateKey
	adjustFactor     float64
	previousSuccess  bool
	delay            time.Duration
	ticker           *time.Ticker
	dataSource       <-chan []byte
	primeUnitCreated chan<- int
	dagFinished      chan<- struct{}
	done             chan struct{}
	log              zerolog.Logger
	wg               sync.WaitGroup
}

// NewService constructs a creating service for the given dag with the given configuration.
// The service creates units with self-adjusting delay. It aims to create units as quickly as possible, while creating only prime units.
// Whenever a prime unit is created, the delay is decreased (multiplying by an adjustment factor).
// Whenever a non-prime unit is created, the delay is increased (dividing by an adjustment factor).
// Whenever two consecutive units are not prime, the adjustment factor is increased (by a constant ratio positiveJerk)
// Whenever a prime unit is created after a non-prime one, the adjustment factor is decreased (by a constant ratio negativeJerk)
// negativeJerk is intentionally stronger than positiveJerk, to encourage convergence.
// The service will close the dagFinished channel when it stops.
func NewService(dag gomel.Dag, adder gomel.Adder, randomSource gomel.RandomSource, config *process.Create, dagFinished chan<- struct{}, dataSource <-chan []byte, log zerolog.Logger) process.Service {
	return &service{
		dag:             dag,
		adder:           adder,
		randomSource:    randomSource,
		pid:             config.Pid,
		maxParents:      config.MaxParents,
		primeOnly:       config.PrimeOnly,
		canSkipLevel:    config.CanSkipLevel,
		maxLevel:        config.MaxLevel,
		privKey:         config.PrivateKey,
		adjustFactor:    config.AdjustFactor,
		previousSuccess: false,
		delay:           config.InitialDelay,
		ticker:          time.NewTicker(config.InitialDelay),
		dataSource:      dataSource,
		dagFinished:     dagFinished,
		done:            make(chan struct{}),
		log:             log,
	}
}

func (s *service) Start() error {
	s.wg.Add(1)
	go func() {
		defer s.ticker.Stop()
		defer s.wg.Done()
		s.createUnit()
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				if !s.createUnit() {
					close(s.dagFinished)
					<-s.done
					return
				}
			}
		}
	}()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	close(s.done)
	s.wg.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}

func (s *service) slower() {
	if !s.previousSuccess {
		s.adjustFactor *= positiveJerk
	}
	s.previousSuccess = false
	s.delay = time.Duration(float64(s.delay) * (1 + s.adjustFactor))
	s.updateTicker()
}

func (s *service) quicker() {
	if !s.previousSuccess {
		s.adjustFactor *= negativeJerk
	}
	s.previousSuccess = true
	s.delay = time.Duration(float64(s.delay) / (1 + s.adjustFactor))
	s.updateTicker()
}

func (s *service) updateTicker() {
	s.ticker.Stop()
	s.ticker = time.NewTicker(s.delay)
}

func (s *service) getData() []byte {
	select {
	case data := <-s.dataSource:
		return data
	default:
		return []byte{}
	}
}

// createUnit creates a unit and adds it to the dag
// It returns boolean value: wheather we can create more units or not.
func (s *service) createUnit() bool {
	var (
		created gomel.Preunit
		level   int
		isPrime bool
		err     error
	)
	if !s.canSkipLevel {
		created, level, err = creating.NewNonSkippingUnit(s.dag, s.pid, s.getData(), s.randomSource)
		isPrime = true
	} else {
		created, level, isPrime, err = creating.NewUnit(s.dag, s.pid, s.maxParents, s.getData(), s.randomSource, s.primeOnly)
	}
	if err != nil {
		s.slower()
		s.log.Info().Msg(logging.NotEnoughParents)
		return true
	}
	created.SetSignature(s.privKey.Sign(created))

	canCreateMore := true
	err = s.adder.AddUnit(created)
	if err != nil {
		s.log.Error().Str("where", "dag.AddUnit callback").Msg(err.Error())
		return canCreateMore
	}

	if isPrime {
		s.log.Info().Int(logging.NParents, len(created.Parents())).Msg(logging.PrimeUnitCreated)
		s.quicker()
	} else {
		s.log.Info().Int(logging.NParents, len(created.Parents())).Msg(logging.UnitCreated)
		s.slower()
	}

	if level >= s.maxLevel {
		canCreateMore = false
	}
	return canCreateMore
}
