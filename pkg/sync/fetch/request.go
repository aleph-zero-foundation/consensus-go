package fetch

import gomel "gitlab.com/alephledger/consensus-go/pkg"

// Request represents a request to ask Pid about Hashes.
type Request struct {
	Pid    uint16
	Hashes []*gomel.Hash
}
