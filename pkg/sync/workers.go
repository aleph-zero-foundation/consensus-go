package sync

import (
	"sync"
	"sync/atomic"
)

// WorkerPool represents a pool of parallel workers.
type WorkerPool interface {
	Start()
	Stop()
}

// pool is a pool of N workers that continuously do the same argument-less work until Stop() is called.
type pool struct {
	size int
	work func()
	wg   sync.WaitGroup
	quit int64
}

// NewPool creates a pool of workers with the given size, all doing the same work.
func NewPool(size int, work func()) WorkerPool {
	return &pool{
		size: size,
		work: work,
	}
}

func (p *pool) Start() {
	p.wg.Add(p.size)
	for i := 0; i < p.size; i++ {
		go func() {
			defer p.wg.Done()
			for atomic.LoadInt64(&p.quit) == 0 {
				p.work()
			}
		}()
	}
}

func (p *pool) Stop() {
	atomic.StoreInt64(&p.quit, 1)
	p.wg.Wait()
}

// perPidPool is a pool of workers that perform work(pid uint16) for all values of id from 0 to nProc-1.
// Each id value can be performed by multiple workers.
type perPidPool struct {
	nProc    uint16
	multiple int
	work     func(id uint16)
	wg       sync.WaitGroup
	quit     int64
}

// NewPerPidPool creates a pool of workers doing per-pid work for the given nProc.
func NewPerPidPool(nProc uint16, multiple int, work func(pid uint16)) WorkerPool {
	return &perPidPool{
		nProc:    nProc,
		multiple: multiple,
		work:     work,
	}
}

func (p *perPidPool) Start() {
	p.wg.Add(p.multiple * int(p.nProc))
	for i := uint16(0); i < p.nProc; i++ {
		for j := 0; j < p.multiple; j++ {
			i := i
			go func() {
				defer p.wg.Done()
				for atomic.LoadInt64(&p.quit) == 0 {
					p.work(i)
				}
			}()
		}
	}
}

func (p *perPidPool) Stop() {
	atomic.StoreInt64(&p.quit, 1)
	p.wg.Wait()
}
