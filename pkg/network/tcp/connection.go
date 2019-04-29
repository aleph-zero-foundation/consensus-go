package tcp

import (
	"net"
)

type conn struct {
	link  *net.TCPConn
	inUse *mutex
}

func newConn(link *net.TCPConn, m *mutex) *conn {
	return &conn{
		link:  link,
		inUse: m,
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
