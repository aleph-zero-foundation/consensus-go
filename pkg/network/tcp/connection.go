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
	sent  uint32
	recv  uint32
	log   zerolog.Logger
}

func newConn(link *net.TCPConn, m *mutex, sent, recv uint32, log zerolog.Logger) *conn {
	return &conn{
		link:  link,
		inUse: m,
		sent:  sent,
		recv:  recv,
		log:   log,
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

func (c *conn) Close() error {
	defer c.inUse.release()
	err := c.link.Close()
	c.log.Info().Uint32(logging.Sent, c.sent).Uint32(logging.Recv, c.recv).Msg(logging.ConnectionClosed)
	return err
}

func (c *conn) TimeoutAfter(t time.Duration) {
	c.link.SetDeadline(time.Now().Add(t))
}

func (c *conn) Log() zerolog.Logger {
	return c.log
}
