package network

import "net"

// Dialer establishes connections with commitee members.
type Dialer interface {

	// Dial connects to the committee member identified by pid and returns the resulting connection or an error.
	Dial(pid uint16) (net.Conn, error)

	// Length returns the number of addresses handled by this dialer.
	Length() int
}
