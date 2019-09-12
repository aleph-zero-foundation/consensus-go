package sync

import (
	"sync"
	"sync/atomic"
)

// Pool represents a pool of parallel workers.
type Pool struct {
	size int
	work func()
	wg   sync.WaitGroup
	quit int32
}

//NewPool creates a pool of workers with the given size, all doing the same work.
func NewPool(size int, work func()) *Pool {
	return &Pool{
		size: size,
		work: work,
	}
}

// Start the pool.
func (p *Pool) Start() {
	p.wg.Add(p.size)
	for i := 0; i < p.size; i++ {
		go func() {
			defer p.wg.Done()
			for {
				if atomic.LoadInt32(&p.quit) > 0 {
					return
				}
				p.work()
			}
		}()
	}
}

// Stop the pool.
func (p *Pool) Stop() {
	atomic.StoreInt32(&p.quit, 1)
	p.wg.Wait()
}
