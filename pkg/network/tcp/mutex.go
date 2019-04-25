package tcp

import "sync/atomic"

type mutex struct {
	token *uint32
}

func (m mutex) tryAcquire() bool {
	return atomic.CompareAndSwapUint32(m.token, 0, 1)
}

func (m mutex) release() {
	atomic.StoreUint32(m.token, 0)
}
