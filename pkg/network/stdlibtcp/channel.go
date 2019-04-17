package stdlibtcp

import (
	"net"
	"sync"
	"sync/atomic"
)

type channel struct {
	connection net.TCPConn
	locker     *uint32
	activate   sync.Once
	localAddr  string
	remoteAddr string
}

func newChannel(localAddr, remoteAddr string) *channel {
	return &channel{
		locker:     new(uint32),
		activate:   sync.Once,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}
}

func (c *channel) Read(b []byte) (int, error) {
	return -1, nil
}

func (c *channel) Write(b []byte) (int, error) {
	return -1, nil
}

func (c *channel) Close() error {
	return nil
}

func (c *channel) tryAcquire() bool {
	return atomic.CompareAndSwapUint32(c.l, 0, 1)
}

func (c *channel) release() {
	atomic.StoreUint32(c.l, 0)
}
