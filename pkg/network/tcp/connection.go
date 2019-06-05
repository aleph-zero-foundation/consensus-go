package tcp

import (
	"net"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

type conn struct {
	link  *net.TCPConn
	inUse *mutex
	pid   uint16
	sid   uint32
	sent  uint32
	recv  uint32
}

func newConn(link *net.TCPConn, m *mutex, pid uint16, sid, sent, recv uint32) *conn {
	return &conn{
		link:  link,
		inUse: m,
		pid:   pid,
		sid:   sid,
		sent:  sent,
		recv:  recv,
	}
}

func (c *conn) Read(b []byte) (int, error) {
	n, err := c.link.Read(b)
	c.recv += uint32(n)
	return n, err
}

func (c *conn) Write(b []byte) (int, error) {
	n, err := c.link.Write(b)
	c.sent += uint32(n)
	return n, err
}

func (c *conn) Close(log zerolog.Logger) error {
	defer c.inUse.release()
	err := c.link.Close()
	log.Info().Uint32(logging.Sent, c.sent).Uint32(logging.Recv, c.recv).Msg(logging.ConnectionClosed)
	return err
}

func (c *conn) TimeoutAfter(t time.Duration) {
	c.link.SetDeadline(time.Now().Add(t))
}

func (c *conn) Pid() uint16 {
	return c.pid
}

func (c *conn) Sid() uint32 {
	return c.sid
}
