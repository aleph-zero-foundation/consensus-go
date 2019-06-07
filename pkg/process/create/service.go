package create

import (
	"sync"
	"time"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
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
	previousSuccess  bool
	delay            time.Duration
	ticker           *time.Ticker
	dataSource       <-chan []byte
	primeUnitCreated chan<- int
	posetFinished    chan<- struct{}
	done             chan struct{}
	log              zerolog.Logger
	wg               sync.WaitGroup
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
	return &service{
		poset:            poset,
		pid:              config.Pid,
		maxParents:       config.MaxParents,
		maxLevel:         config.MaxLevel,
		maxHeight:        config.MaxHeight,
		privKey:          config.PrivateKey,
		adjustFactor:     config.AdjustFactor,
		previousSuccess:  false,
		delay:            config.InitialDelay,
		ticker:           time.NewTicker(config.InitialDelay),
		dataSource:       dataSource,
		primeUnitCreated: primeUnitCreated,
		posetFinished:    posetFinished,
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
	created, err := creating.NewUnit(s.poset, s.pid, s.maxParents, s.getData())
	if err != nil {
		s.slower()
		s.log.Info().Msg(logging.NotEnoughParents)
		return
	}
	created.SetSignature(s.privKey.Sign(created))

	if len(created.Parents()) == 0 {
		tc, err := tcoin.Decode(created.ThresholdCoinData(), s.pid)
		if err != nil {
			s.log.Error().Str("where", "poset.createUnit.tcoin.Decode").Msg(err.Error())
			return
		}
		s.poset.AddThresholdCoin(created.Hash(), tc)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	s.poset.AddUnit(created, func(_ gomel.Preunit, added gomel.Unit, err error) {
		defer wg.Done()
		if err != nil {
			s.poset.RemoveThresholdCoin(added.Hash())
			s.log.Error().Str("where", "poset.AddUnit callback").Msg(err.Error())
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

		if added.Level() >= s.maxLevel || added.Height() >= s.maxHeight {
			s.ticker.Stop()
			close(s.posetFinished)
		}
	})
	wg.Wait()
}
