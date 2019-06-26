package tcp

import (
	"bufio"
	"net"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

type conn struct {
	link   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	sent   uint32
	recv   uint32
	log    zerolog.Logger
}

func NewConn(link net.Conn, sent, recv uint32, log zerolog.Logger) *conn {
	return &conn{
		link:   link,
		reader: bufio.NewReader(link),
		writer: bufio.NewWriter(link),
		sent:   sent,
		recv:   recv,
		log:    log,
	}
}

func (c *conn) Read(b []byte) (int, error) {
	n, err := c.reader.Read(b)
	c.recv += uint32(n)
	return n, err
}

func (c *conn) Write(b []byte) (int, error) {
	written, n := 0, 0
	var err error
	for written < len(b) {
		n, err = c.writer.Write(b[written:])
		written += n
		if err == bufio.ErrBufferFull {
			err = c.writer.Flush()
		}
		if err != nil {
			break
		}
	}
	c.sent += uint32(written)
	return written, err
}

func (c *conn) Flush() error {
	return c.writer.Flush()
}

func (c *conn) Close() error {
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

func (c *conn) SetLogger(log zerolog.Logger) {
	c.log = log
}
