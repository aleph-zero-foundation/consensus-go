package udp

import (
	"errors"
	"net"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

type outConn struct {
	link        net.Conn
	writeBuffer []byte
	sent        uint32
	log         zerolog.Logger
}

func newOutConn(link net.Conn, log zerolog.Logger) *outConn {
	return &outConn{
		link:        link,
		writeBuffer: make([]byte, 0),
		sent:        0,
		log:         log,
	}
}

func (c *outConn) Read(b []byte) (int, error) {
	return 0, errors.New("cannot read from outgoing UDP connection")
}

func (c *outConn) Write(b []byte) (int, error) {
	if len(c.writeBuffer)+len(b) >= (1<<16)-512 {
		return 0, errors.New("cannot write as the message length would exceed 65023, did you forget to Flush()?")
	}
	c.writeBuffer = append(c.writeBuffer, b...)
	return len(b), nil
}

func (c *outConn) Flush() error {
	_, err := c.link.Write(c.writeBuffer)
	c.sent += uint32(len(c.writeBuffer))
	c.writeBuffer = make([]byte, 0)
	return err
}

func (c *outConn) Close() error {
	err := c.link.Close()
	c.log.Info().Uint32(logging.Sent, c.sent).Msg(logging.ConnectionClosed)
	return err
}

func (c *outConn) TimeoutAfter(t time.Duration) {
	c.link.SetDeadline(time.Now().Add(t))
}

func (c *outConn) Log() zerolog.Logger {
	return c.log
}
