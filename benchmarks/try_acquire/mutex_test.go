package mutex_test

import (
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

type softMutex interface {
	tryAcquire() bool
	relase()
}

type chanMutex struct {
	token chan struct{}
}

func (m *chanMutex) tryAcquire() bool {
	select {
	case _, ok := <-m.token:
		return ok
	default:
		return false
	}
}

func (m *chanMutex) relase() {
	m.token <- struct{}{}
}

func newChanMutex() softMutex {
	m := chanMutex{make(chan struct{}, 1)}
	m.token <- struct{}{}
	return &m
}

type atomMutex struct {
	token int32
}

func (m *atomMutex) tryAcquire() bool {
	return atomic.CompareAndSwapInt32(&m.token, 0, 1)
}

func (m *atomMutex) relase() {
	atomic.StoreInt32(&m.token, 0)
}

func newAtomMutex() softMutex {
	return &atomMutex{}
}

func BenchmarkChannelRealistic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(newChanMutex, 10, 1024)
	}
}

func BenchmarkAtomicRealistic(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(newAtomMutex, 10, 1024)
	}
}

func BenchmarkChannelCrowded(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(newChanMutex, 100, 1024)
	}
}

func BenchmarkAtomicCrowded(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(newAtomMutex, 100, 1024)
	}
}

func BenchmarkChannelTooMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(newChanMutex, 100, 128)
	}
}

func BenchmarkAtomicTooMany(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(newAtomMutex, 100, 128)
	}
}

func run(newMutex func() softMutex, nRout, nMut int) {
	var mxs []softMutex
	for i := 0; i < nMut; i++ {
		mxs = append(mxs, newMutex())
	}
	var wg sync.WaitGroup
	wg.Add(nRout)
	for i := 0; i < nRout; i++ {
		go play(mxs, &wg)
	}
	wg.Wait()
}

func play(mxs []softMutex, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < 100000; i++ {
		mx := mxs[rand.Intn(len(mxs))]
		if mx.tryAcquire() {
			busyIO()
			mx.relase()
		}
	}
}

func busyIO() {
	f, err := os.OpenFile("/dev/null", os.O_WRONLY, 666)
	if err != nil {
		panic("oh boy")
	}
	defer f.Close()
	for i := 0; i < 10; i++ {
		data := make([]byte, 43)
		f.Write(data)
	}
}
