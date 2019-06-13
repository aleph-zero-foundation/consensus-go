package sync

import (
	"bufio"
	"net"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Connection represents a connection between two processes.
type Connection interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Flush() error
	Close() error
	TimeoutAfter(t time.Duration)
	Log() zerolog.Logger
}

type conn struct {
	link   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	inUse  *Mutex
	sent   uint32
	recv   uint32
	log    zerolog.Logger
}

func newConn(link net.Conn, m *Mutex, sent, recv uint32, log zerolog.Logger) *conn {
	return &conn{
		link:   link,
		reader: bufio.NewReader(link),
		writer: bufio.NewWriter(link),
		inUse:  m,
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
	defer c.inUse.Release()
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
