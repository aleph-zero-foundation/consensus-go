package create

import (
	"sync"
	"time"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
)

// adjustingCreator creates units with a self-adjusting delay. It aims to create units as quickly as possible, while creating only prime units.
// Whenever it creates a prime unit it lowers the delay by multiplying it by an adjustment factor.
// Whenever it creates a non-prime unit it increases the delay by dividing it by the adjustment factor.
// Whenever it creates a prime unit after creating a non-prime unit it decreases the delay adjustment factor before lowering the delay.
// Whenever it creates a non-prime unit after creating a non-prime unit it increases the delay adjustment factor before increasing the delay.
// The deacreasing of the adjustment factor is intentionally stronger than the increasing, to encourage convergence.
type adjustingCreator struct {
	currentDelay time.Duration
	adjustFactor float64
	ticker       *time.Ticker
	poset        gomel.Poset
	id           int
	maxParents   int
	privKey      gomel.PrivateKey
	final        func(gomel.Unit) bool
	lastAttempt  int
	done         chan struct{}
}

func newAdjustingCreator(poset gomel.Poset, id, maxParents int, privKey gomel.PrivateKey, delay int, final func(gomel.Unit) bool) *adjustingCreator {
	initialDelay := time.Duration(delay) * time.Millisecond
	return &adjustingCreator{
		currentDelay: initialDelay,
		adjustFactor: 0.14,
		ticker:       time.NewTicker(initialDelay),
		poset:        poset,
		id:           id,
		maxParents:   maxParents,
		privKey:      privKey,
		final:        final,
		lastAttempt:  0,
		done:         make(chan struct{}),
	}
}

func (ac *adjustingCreator) slower() {
	if ac.lastAttempt == 0 {
		ac.adjustFactor *= 1.01
	}
	ac.lastAttempt = 0
	ac.currentDelay = time.Duration(float64(ac.currentDelay) * (1 + ac.adjustFactor))
	ac.updateTicker()
}

func (ac *adjustingCreator) quicker() {
	if ac.lastAttempt == 0 {
		ac.adjustFactor *= 0.9
	}
	ac.lastAttempt = 1
	ac.currentDelay = time.Duration(float64(ac.currentDelay) / (1 + ac.adjustFactor))
	ac.updateTicker()
}

func (ac *adjustingCreator) updateTicker() {
	ac.ticker.Stop()
	ac.ticker = time.NewTicker(ac.currentDelay)
}

func (ac *adjustingCreator) createUnit() {
	// TODO: actually get transactions from somewhere
	created, err := creating.NewUnit(ac.poset, ac.id, ac.maxParents, []gomel.Tx{})
	if err != nil {
		ac.slower()
		return
	}
	created.SetSignature(ac.privKey.Sign(created))
	var wg sync.WaitGroup
	wg.Add(1)
	ac.poset.AddUnit(created, func(_ gomel.Preunit, added gomel.Unit, err error) {
		defer wg.Done()
		if err != nil {
			// TODO: error handling
			return
		}
		if gomel.Prime(added) {
			ac.quicker()
		} else {
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
