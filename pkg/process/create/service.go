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
	dag              gomel.Dag
	randomSource     gomel.RandomSource
	pid              int
	maxParents       int
	primeOnly        bool
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
// The service will close dagFinished channel when it stops.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Create, dagFinished chan<- struct{}, primeUnitCreated chan<- int, dataSource <-chan []byte, log zerolog.Logger) (process.Service, error) {
	return &service{
		dag:              dag,
		randomSource:     randomSource,
		pid:              config.Pid,
		maxParents:       config.MaxParents,
		primeOnly:        config.PrimeOnly,
		maxLevel:         config.MaxLevel,
		privKey:          config.PrivateKey,
		adjustFactor:     config.AdjustFactor,
		previousSuccess:  false,
		delay:            config.InitialDelay,
		ticker:           time.NewTicker(config.InitialDelay),
		dataSource:       dataSource,
		primeUnitCreated: primeUnitCreated,
		dagFinished:      dagFinished,
		done:             make(chan struct{}),
		log:              log,
	}, nil
}

func (s *service) Start() error {
	s.wg.Add(1)
	go func() {
		s.createUnit()
		for {
			select {
			case <-s.done:
				s.ticker.Stop()
				s.wg.Done()
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

func (s *service) createUnit() {
	created, err := creating.NewUnit(s.dag, s.pid, s.maxParents, s.getData(),
		s.randomSource, s.primeOnly)
	if err != nil {
		s.slower()
		s.log.Info().Msg(logging.NotEnoughParents)
		return
	}
	created.SetSignature(s.privKey.Sign(created))

	var wg sync.WaitGroup
	wg.Add(1)
	s.dag.AddUnit(created, s.randomSource, func(_ gomel.Preunit, added gomel.Unit, err error) {
		defer wg.Done()
		if err != nil {
			s.log.Error().Str("where", "dag.AddUnit callback").Msg(err.Error())
			return
		}

		if gomel.Prime(added) {
			s.log.Info().Int(logging.Height, added.Height()).Int(logging.NParents, len(added.Parents())).Msg(logging.PrimeUnitCreated)
			s.quicker()
			s.primeUnitCreated <- added.Level()
		} else {
			s.log.Info().Int(logging.Height, added.Height()).Int(logging.NParents, len(added.Parents())).Msg(logging.UnitCreated)
			s.slower()
		}

		if added.Level() >= s.maxLevel {
			s.ticker.Stop()
			close(s.dagFinished)
		}
	})
	wg.Wait()
}
