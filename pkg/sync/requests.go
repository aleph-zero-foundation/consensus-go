package sync

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

// Gossip is a function that initializes gossip with the given PID.
type Gossip func(uint16)

// Fetch is a function that contacts the given PID and requests units with given IDs.
type Fetch func(uint16, []uint64)

// Multicast is a function that sends the given unit to all committee members.
type Multicast func(gomel.Unit)
