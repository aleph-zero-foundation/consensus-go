package network

import (
	"io"
	"net"
)

// Dialer establishes connections with commitee members.
type Dialer interface {

	// Dial connects to the committee member identified by pid and returns the resulting connection or an error.
	Dial(pid uint16) (net.Conn, error)

	// DialAll returns a writer that can be used to multicast messages to all the committee members.
	DialAll() io.Writer

	// Length returns the number of addresses handled by this dialer.
	Length() int
}
