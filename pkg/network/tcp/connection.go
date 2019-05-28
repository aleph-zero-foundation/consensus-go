package tcp

import (
	"net"
	"time"
)

type conn struct {
	link  *net.TCPConn
	inUse *mutex
	sid   uint32
}

func newConn(link *net.TCPConn, m *mutex, sid uint32) *conn {
	return &conn{
		link:  link,
		inUse: m,
		sid:   sid,
	}
}

func (c *conn) Read(b []byte) (int, error) {
	return c.link.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	return c.link.Write(b)
}

func (c *conn) Close() error {
	defer c.inUse.release()
	return c.link.Close()
}

func (c *conn) TimeoutAfter(t time.Duration) {
	c.link.SetDeadline(time.Now().Add(t))
}

func (c *conn) Sid() uint32 {
	return c.sid
}
