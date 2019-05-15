package sync

import (
	"math/rand"
	"time"
)

// dialer is a simple implementation, but it should be fine for now
type dialer struct {
	n      int
	source chan int
	ticker *time.Ticker
	done   chan struct{}
}

func newDialer(n int, syncInitDelay int) *dialer {
	return &dialer{
		n:      n,
		source: make(chan int),
		ticker: time.NewTicker(time.Duration(syncInitDelay) * time.Millisecond),
		done:   make(chan struct{}),
	}
}

func (d *dialer) channel() <-chan int {
	return d.source
}

func (d *dialer) start() {
	go func() {
		for range d.ticker.C {
			n := rand.Intn(d.n)
			select {
			case d.source <- n:
			case <-d.done:
				close(d.source)
				d.ticker.Stop()
				return
			}
		}
	}()
}

func (d *dialer) stop() {
	close(d.done)
}
