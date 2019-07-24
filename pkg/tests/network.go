package tests

import (
	"errors"
	"io"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type connection struct {
	in  *io.PipeReader
	out *io.PipeWriter
}

func (c *connection) Read(buf []byte) (int, error) {
	return c.in.Read(buf)
}

func (c *connection) Write(buf []byte) (int, error) {
	return c.out.Write(buf)
}

func (c *connection) Flush() error {
	return nil
}

func (c *connection) Close() error {
	if err := c.in.CloseWithError(errors.New("")); err != nil {
		c.out.CloseWithError(errors.New(""))
		return err
	}
	return c.out.CloseWithError(errors.New(""))
}

func (c *connection) TimeoutAfter(time.Duration) {}

func (c *connection) Log() zerolog.Logger {
	return zerolog.Nop()
}

func (c *connection) SetLogger(zerolog.Logger) {}

// NewConnection creates a pipe simulating a pair of network connections.
func NewConnection() (network.Connection, network.Connection) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &connection{r1, w2}, &connection{r2, w1}
}

// Dialer implements network.Dialer and has an additional method for closing it.
type Dialer struct {
	dialChan []chan<- network.Connection
}

// Dial creates a new connection, pushes one end to the associated listener and return the other.
func (d *Dialer) Dial(k uint16) (network.Connection, error) {
	out, in := NewConnection()
	d.dialChan[k] <- in
	return out, nil
}

// Close makes all listeners associated with this dialer return errors.
func (d *Dialer) Close() {
	for _, ch := range d.dialChan {
		close(ch)
	}
}

type listener struct {
	listenChan <-chan network.Connection
}

func (l *listener) Listen(_ time.Duration) (network.Connection, error) {
	conn, ok := <-l.listenChan
	if !ok {
		return nil, errors.New("done")
	}
	return conn, nil
}

// NewNetwork returns a dialer and a slice of listeners. When the dialer is used,
// it returns a connection corresponding to an endpoint that has been pushed to the corresponding listener.
// This is not suitable for bigger tests, unfortunately, some form of combining listeners might be needed.
func NewNetwork(length int) (*Dialer, []network.Listener) {
	chans := make([]chan<- network.Connection, length)
	listeners := make([]network.Listener, length)
	for i := range listeners {
		locChan := make(chan network.Connection)
		chans[i] = locChan
		listeners[i] = &listener{locChan}
	}
	return &Dialer{chans}, listeners
}
