package create

import (
	"sync"
	"time"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/process"
)

const (
	positiveJerk = 1.01
	negativeJerk = 0.90
)

type service struct {
	poset            gomel.Poset
	pid              int
	maxParents       int
	maxLevel         int
	maxHeight        int
	privKey          gomel.PrivateKey
	adjustFactor     float64
	previousPrime    bool
	delay            time.Duration
	ticker           *time.Ticker
	dataSource       <-chan []byte
	primeUnitCreated chan<- int
	posetFinished    chan<- struct{}
	done             chan struct{}
	log              zerolog.Logger
}

// NewService constructs a creating service for the given poset with the given configuration.
// The service creates units with self-adjusting delay. It aims to create units as quickly as possible, while creating only prime units.
// Whenever a prime unit is created, the delay is decreased (multiplying by an adjustment factor).
// Whenever a non-prime unit is created, the delay is increased (dividing by an adjustment factor).
// Whenever two consecutive units are not prime, the adjustment factor is increased (by a constant ratio positiveJerk)
// Whenever a prime unit is created after a non-prime one, the adjustment factor is decreased (by a constant ratio negativeJerk)
// negativeJerk is intentionally stronger than positiveJerk, to encourage convergence.
// The service will close posetFinished channel when it stops.
func NewService(poset gomel.Poset, config *process.Create, posetFinished chan<- struct{}, primeUnitCreated chan<- int, dataSource <-chan []byte, log zerolog.Logger) (process.Service, error) {
	initialDelay := time.Duration(config.InitialDelay) * time.Millisecond

	return &service{
		poset:            poset,
		pid:              config.Pid,
		maxParents:       config.MaxParents,
		maxLevel:         config.MaxLevel,
		maxHeight:        config.MaxHeight,
		privKey:          config.PrivateKey,
		adjustFactor:     config.AdjustFactor,
		previousPrime:    false,
		delay:            initialDelay,
		ticker:           time.NewTicker(initialDelay),
		dataSource:       dataSource,
		primeUnitCreated: primeUnitCreated,
		posetFinished:    posetFinished,
		done:             make(chan struct{}),
		log:              log,
	}, nil
}

func (s *service) Start() error {
	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				s.createUnit()
			}
		}
	}()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.ticker.Stop()
	close(s.done)
	s.log.Info().Msg(logging.ServiceStopped)

}

func (s *service) slower() {
	if !s.previousPrime {
		s.adjustFactor *= positiveJerk
	}
	s.delay = time.Duration(float64(s.delay) * (1 + s.adjustFactor))
	s.updateTicker()
}

func (s *service) quicker() {
	if !s.previousPrime {
		s.adjustFactor *= negativeJerk
	}
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

func (s *service) createUnit() {
	created, err := creating.NewUnit(s.poset, s.pid, s.maxParents, s.getData())
	if err != nil {
		s.slower()
		s.log.Info().Msg(logging.NotEnoughParents)
		return
	}
	created.SetSignature(s.privKey.Sign(created))

	var wg sync.WaitGroup
	wg.Add(1)
	s.poset.AddUnit(created, func(_ gomel.Preunit, added gomel.Unit, err error) {
		defer wg.Done()
		if err != nil {
			s.log.Error().Msg(err.Error())
			return
		}

		if gomel.Prime(added) {
			s.log.Info().Msg(logging.PrimeUnitCreated)
			s.quicker()
			s.previousPrime = true
			s.primeUnitCreated <- added.Level()
		} else {
			s.log.Info().Msg(logging.UnitCreated)
			s.slower()
			s.previousPrime = false
		}

		if added.Level() >= s.maxLevel || added.Height() >= s.maxHeight {
			s.ticker.Stop()
			close(s.posetFinished)
		}
	})
	wg.Wait()
}
