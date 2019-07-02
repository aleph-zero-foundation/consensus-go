package network

// Dialer establishes connections with committee members.
type Dialer interface {

	// Dial connects to the committee member identified by pid and returns the resulting connection or an error.
	Dial(pid uint16) (Connection, error)

	// DialAll returns a writer that can be used to multicast messages to all the committee members.
	DialAll() (*Multicaster, error)

	// Length returns the number of addresses handled by this dialer.
	Length() int
}
