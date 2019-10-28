package gomel

import (
	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
)

// Data is a packet of binary data that is embedded in a single unit.
type Data []byte

// DataSource is a source of units data.
type DataSource <-chan Data

// Preblock is a set of Data objects from units contained in one block (timing round).
type Preblock struct {
	data        []Data
	randomBytes []byte
}

// PreblockSink is an output of the aleph protocol.
type PreblockSink chan<- *Preblock

// NewPreblock constructs a preblock from given data and randomBytes.
func NewPreblock(data []Data, randomBytes []byte) *Preblock {
	return &Preblock{data, randomBytes}
}

// ToPreblock extracts preblock from a given timing round.
// It assumes that
// 1. given slice of units forms a timing round,
// 2. timing unit is the last unit in the slice,
// 3. random source data of the timing unit starts with
// random bytes from the previous level.
func ToPreblock(round []Unit) *Preblock {
	data := make([]Data, 0, len(round))
	for _, u := range round {
		data = append(data, u.Data())
	}
	randomBytes := round[len(round)-1].RandomSourceData()[:bn256.SignatureLength]
	return &Preblock{data, randomBytes}
}
