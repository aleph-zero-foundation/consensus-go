package udp

import (
	"bytes"
	"errors"
	"io"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type connIn struct {
	reader io.Reader
	recv   uint32
	log    zerolog.Logger
}

//newConnIn initializes an incoming UDP "connection" -- wrapping the content of the incoming packet
func newConnIn(packet []byte, log zerolog.Logger) network.Connection {
	return &connIn{
		reader: bytes.NewReader(packet),
		recv:   0,
		log:    log,
	}
}

func (c *connIn) Read(b []byte) (int, error) {
	n, err := c.reader.Read(b)
	c.recv += uint32(n)
	return n, err
}

func (c *connIn) Write(b []byte) (int, error) {
	return 0, errors.New("cannot write to incoming UDP connection")
}

func (c *connIn) Flush() error {
	return errors.New("cannot flush incoming UDP connection")
}

func (c *connIn) Close() error {
	return nil
}

func (c *connIn) TimeoutAfter(t time.Duration) {
	// does nothing as the UDP connIn is non-blocking anyway
}

func (c *connIn) Log() zerolog.Logger {
	return c.log
}

func (c *connIn) SetLogger(log zerolog.Logger) {
	c.log = log
}
