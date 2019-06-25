package udp

import (
	"bytes"
	"errors"
	"io"
	"time"

	"github.com/rs/zerolog"
)

type inConn struct {
	reader io.Reader
	recv   uint32
	log    zerolog.Logger
}

func newInConn(packet []byte, log zerolog.Logger) *inConn {
	return &inConn{
		reader: bytes.NewReader(packet),
		recv:   0,
		log:    log,
	}
}

func (c *inConn) Read(b []byte) (int, error) {
	n, err := c.reader.Read(b)
	c.recv += uint32(n)
	return n, err
}

func (c *inConn) Write(b []byte) (int, error) {
	return 0, errors.New("cannot write to incoming UDP connection")
}

func (c *inConn) Flush() error {
	return errors.New("cannot flush incoming UDP connection")
}

func (c *inConn) Close() error {
	return nil
}

func (c *inConn) TimeoutAfter(t time.Duration) {
	// does nothing as the UDP inConn is non-blocking anyway
}

func (c *inConn) Log() zerolog.Logger {
	return c.log
}
