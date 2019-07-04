package sync

import (
	"sync"
	"sync/atomic"
)

type pool struct {
	size uint
	work func()
	wg   sync.WaitGroup
	quit int32
}

func newPool(size uint, work func()) *pool {
	return &pool{
		size: size,
		work: work,
	}
}

func (p *pool) start() {
	p.wg.Add(int(p.size))
	for i := uint(0); i < p.size; i++ {
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

func (p *pool) stop() {
	atomic.StoreInt32(&p.quit, 1)
	p.wg.Wait()
}
