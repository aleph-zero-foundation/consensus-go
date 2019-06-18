package sync

import (
	"math/rand"
	"sync"
	"time"
)

// dialer is a simple implementation, but it should be fine for now
type dialer struct {
	n      int
	id     int
	source chan uint16
	ticker *time.Ticker
	done   chan struct{}
	wg     sync.WaitGroup
}

func newDialer(n, id int, syncInitDelay time.Duration) *dialer {
	return &dialer{
		n:      n,
		id:     id,
		source: make(chan uint16),
		ticker: time.NewTicker(syncInitDelay),
		done:   make(chan struct{}),
	}
}

func (d *dialer) channel() <-chan uint16 {
	return d.source
}

func (d *dialer) start() {
	d.wg.Add(1)
	go func() {
		for range d.ticker.C {
			n := rand.Intn(d.n)
			for n == d.id {
				n = rand.Intn(d.n)
			}
			select {
			case d.source <- uint16(n):
			case <-d.done:
				close(d.source)
				d.ticker.Stop()
				d.wg.Done()
				return
			}
		}
	}()
}

func (d *dialer) stop() {
	close(d.done)
	d.wg.Wait()
}
