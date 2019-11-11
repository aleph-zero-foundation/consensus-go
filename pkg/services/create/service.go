// Package create implements a service for creating new units.
package create

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

type service struct {
	dag          gomel.Dag
	adder        gomel.Adder
	randomSource gomel.RandomSource
	pid          uint16
	primeOnly    bool
	canSkipLevel bool
	maxLevel     int
	privKey      gomel.PrivateKey
	ticker       *time.Ticker
	dataSource   gomel.DataSource
	dagFinished  chan<- struct{}
	quit         int64
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewService constructs a creating service for the given dag with the given configuration.
// The service creates units with self-adjusting delay. It aims to create units as quickly as possible, while creating only prime units.
// Whenever a prime unit is created, the delay is decreased (multiplying by an adjustment factor).
// Whenever a non-prime unit is created, the delay is increased (dividing by an adjustment factor).
// Whenever two consecutive units are not prime, the adjustment factor is increased (by a constant ratio positiveJerk)
// Whenever a prime unit is created after a non-prime one, the adjustment factor is decreased (by a constant ratio negativeJerk)
// negativeJerk is intentionally stronger than positiveJerk, to encourage convergence.
// The service will close the dagFinished channel when it stops.
func NewService(dag gomel.Dag, adder gomel.Adder, randomSource gomel.RandomSource, conf *config.Create, dagFinished chan<- struct{}, dataSource gomel.DataSource, log zerolog.Logger) gomel.Service {
	return &service{
		dag:          dag,
		adder:        adder,
		randomSource: randomSource,
		pid:          conf.Pid,
		primeOnly:    conf.PrimeOnly,
		canSkipLevel: conf.CanSkipLevel,
		maxLevel:     conf.MaxLevel,
		privKey:      conf.PrivateKey,
		ticker:       time.NewTicker(conf.Delay),
		dataSource:   dataSource,
		dagFinished:  dagFinished,
		log:          log,
	}
}

func (s *service) Start() error {
	s.wg.Add(1)
	go func() {
		defer s.ticker.Stop()
		defer s.wg.Done()
		for atomic.LoadInt64(&s.quit) == 0 {
			if !s.createUnit() {
				close(s.dagFinished)
				return
			}
			<-s.ticker.C
		}
	}()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	atomic.StoreInt64(&s.quit, 1)
	s.wg.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
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
// It returns boolean value: whether we can create more units or not.
func (s *service) createUnit() bool {
	created, level, err := creating.NewUnit(s.dag, s.pid, s.getData(), s.randomSource, s.canSkipLevel)
	if err != nil {
		s.log.Info().Msg(logging.NotEnoughParents)
		return true
	}
	created.SetSignature(s.privKey.Sign(created))
	err = s.adder.AddUnit(created, s.pid)
	if err != nil {
		s.log.Error().Str("where", "create.AddUnit").Msg(err.Error())
		return true
	}
	s.log.Info().Int(logging.Height, created.Height()).Msg(logging.UnitCreated)
	return level < s.maxLevel
}
