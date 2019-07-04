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
	return zerolog.Logger{}
}

func (c *connection) SetLogger(zerolog.Logger) {}

// NewConnection creates a pipe simulating a pair of network connections.
func NewConnection() (network.Connection, network.Connection) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &connection{r1, w2}, &connection{r2, w1}
}

type dialer struct {
	out      []network.Connection
	in       []network.Connection
	dialChan []chan<- network.Connection
}

func (d *dialer) Dial(k uint16) (network.Connection, error) {
	d.dialChan[k] <- d.in[k]
	return d.out[k], nil
}

func (d *dialer) DialAll() (*network.Multicaster, error) {
	for i, c := range d.in {
		d.dialChan[i] <- c
	}
	return network.NewMulticaster(d.out), nil
}

func (d *dialer) Length() int {
	return len(d.out)
}

type listener struct {
	listenChan <-chan network.Connection
}

func (l *listener) Listen(_ time.Duration) (network.Connection, error) {
	return <-l.listenChan, nil
}

// NewNetwork returns a dialer and a slice of listeners. When the dialer is used,
// it returns a connection corresponding to an endpoint that has been pushed to the corresponding listener.
// This is not suitable for bigger tests, unfortunately, some form of combining listeners might be needed.
func NewNetwork(length int) (network.Dialer, []network.Listener) {
	outConn, inConn := make([]network.Connection, length), make([]network.Connection, length)
	chans := make([]chan<- network.Connection, length)
	listeners := make([]network.Listener, length)
	for i := range outConn {
		outConn[i], inConn[i] = NewConnection()
		locChan := make(chan network.Connection)
		chans[i] = locChan
		listeners[i] = &listener{locChan}
	}
	return &dialer{outConn, inConn, chans}, listeners
}
