package gomel

// Syncer syncs.
type Syncer interface {
	// RequestGossip with the given committee member.
	RequestGossip(uint16)
	// RequestFetch send a request to the given committee member for units with given IDs.
	RequestFetch(uint16, []uint64)
	// Multicast the unit.
	Multicast(Unit)
}
