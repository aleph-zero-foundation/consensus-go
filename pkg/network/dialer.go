package network

// Dialer establishes connections with committee members.
type Dialer interface {
	// Dial connects to the committee member identified by pid and returns the resulting connection or an error.
	Dial(pid uint16) (Connection, error)
}
