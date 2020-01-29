package gomel

import (
	"gitlab.com/alephledger/consensus-go/pkg/crypto/bn256"
)

// Data is a packet of binary data that is embedded in a single unit.
type Data []byte

// DataSource is a source of units data.
type DataSource <-chan Data

// DataSink is an output for the data to sort.
type DataSink chan<- Data

// Preblock is a set of Data objects from units contained in one block (timing round).
type Preblock struct {
	data        []Data
	randomBytes []byte
}

// PreblockSink is an output of the aleph protocol.
// NOTE: we assume that after call to this function finishes we are free to clean up all data required to generate
// the handled Preblock. After this it is gomel's caller responsibility to handle all requests to any data related
// with this Preblock.
type PreblockSink chan<- func(func(*Preblock))

// PreblockSource is a source of preblocks.
type PreblockSource <-chan *Preblock

// Block is a final element of the blockchain produced by the protocol.
type Block struct {
	Preblock
	// more to come
}

// BlockSource is a source of blocks.
type BlockSource <-chan *Block

// BlockSink is an output channel for the blockchain produced.
type BlockSink chan<- *Block

// NewPreblock constructs a preblock from given data and randomBytes.
func NewPreblock(data []Data, randomBytes []byte) *Preblock {
	return &Preblock{data, randomBytes}
}

// ToBlock creates a block from a given preblock.
func ToBlock(pb *Preblock) *Block {
	return &Block{*pb}
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
