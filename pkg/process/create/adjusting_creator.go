package create

import (
	"sync"
	"time"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

const (
	positiveJerk = 1.01
	negativeJerk = 0.90
)

// adjustingCreator creates units with a self-adjusting delay. It aims to create units as quickly as possible, while creating only prime units.
// Whenever it creates a prime unit it lowers the delay by multiplying it by an adjustment factor.
// Whenever it creates a non-prime unit it increases the delay by dividing it by the adjustment factor.
// Whenever it creates a prime unit after creating a non-prime unit it decreases the delay adjustment factor before lowering the delay.
// Whenever it creates a non-prime unit after creating a non-prime unit it increases the delay adjustment factor before increasing the delay.
// The deacreasing of the adjustment factor is intentionally stronger than the increasing, to encourage convergence.
type adjustingCreator struct {
	delay           time.Duration
	adjustFactor    float64
	ticker          *time.Ticker
	poset           gomel.Poset
	id              int
	maxParents      int
	privKey         gomel.PrivateKey
	final           func(gomel.Unit) bool
	previousSuccess bool
	txpu            uint
	txSource        <-chan *gomel.Tx
	done            chan struct{}
	log             zerolog.Logger
}

func newAdjustingCreator(poset gomel.Poset, id, maxParents int, privKey gomel.PrivateKey, delay int, adjustFactor float64, final func(gomel.Unit) bool, txpu uint, txSource <-chan *gomel.Tx, log zerolog.Logger) *adjustingCreator {
	initialDelay := time.Duration(delay) * time.Millisecond
	return &adjustingCreator{
		delay:           initialDelay,
		adjustFactor:    adjustFactor,
		ticker:          time.NewTicker(initialDelay),
		poset:           poset,
		id:              id,
		maxParents:      maxParents,
		privKey:         privKey,
		final:           final,
		previousSuccess: false,
		txpu:            txpu,
		txSource:        txSource,
		done:            make(chan struct{}),
		log:             log,
	}
}

func (ac *adjustingCreator) slower() {
	if !ac.previousSuccess {
		ac.adjustFactor *= positiveJerk
	}
	ac.previousSuccess = false
	ac.delay = time.Duration(float64(ac.delay) * (1 + ac.adjustFactor))
	ac.updateTicker()
}

func (ac *adjustingCreator) quicker() {
	if !ac.previousSuccess {
		ac.adjustFactor *= negativeJerk
	}
	ac.previousSuccess = true
	ac.delay = time.Duration(float64(ac.delay) / (1 + ac.adjustFactor))
	ac.updateTicker()
}

func (ac *adjustingCreator) updateTicker() {
	ac.ticker.Stop()
	ac.ticker = time.NewTicker(ac.delay)
}

func (ac *adjustingCreator) getTransactions() []gomel.Tx {
	result := []gomel.Tx{}
	for uint(len(result)) < ac.txpu {
		select {
		case tx := <-ac.txSource:
			result = append(result, *tx)
		default:
			return result
		}
	}
	return result
}

func (ac *adjustingCreator) createUnit() {
	txs := ac.getTransactions()
	created, err := creating.NewUnit(ac.poset, ac.id, ac.maxParents, txs)
	if err != nil {
		ac.slower()
		ac.log.Info().Msg(logging.NotEnoughParents)
		return
	}
	created.SetSignature(ac.privKey.Sign(created))
	var wg sync.WaitGroup
	wg.Add(1)
	ac.poset.AddUnit(created, func(_ gomel.Preunit, added gomel.Unit, err error) {
		defer wg.Done()
		if err != nil {
			ac.log.Error().Msg(err.Error())
			return
		}
		if gomel.Prime(added) {
			ac.log.Info().Int("X", len(txs)).Msg(logging.PrimeUnitCreated)
			ac.quicker()
		} else {
			ac.log.Info().Int("X", len(txs)).Msg(logging.UnitCreated)
			ac.slower()
		}
		if ac.final(added) {
			ac.stop()
		}
	})
	wg.Wait()
}

func (ac *adjustingCreator) start() {
	go func() {
		for {
			select {
			case <-ac.done:
				ac.ticker.Stop()
				break
			case <-ac.ticker.C:
				ac.createUnit()
			}
		}
	}()
}

func (ac *adjustingCreator) stop() {
	close(ac.done)
}
