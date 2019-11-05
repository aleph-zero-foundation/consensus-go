// Package encoding implements custom encoding of units.
//
// Note that the objects being encoded are units, while the objects resulting from decoding are preunits.
// This makes perfect sense, as we only want to send information about units we already added to our dag,
// and any decoded information will have to be manually included into the dag.
//
// Crucially, and in contrast to Gob, this encoding only reads the bytes it needs.
package encoding

import (
	"io"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// Encoder extends io.Writer with ability to encode units.
type encoder interface {
	io.Writer
	// encodeCrown encodes a crown.
	encodeCrown(*gomel.Crown) error
	// encodeUnit encodes a single unit.
	encodeUnit(gomel.Unit) error
	// encodeChunk encodes a slice of units by splitting them into antichains.
	encodeChunk([]gomel.Unit) error
}

// Decoder extends io.Reader with ability to encode units.
type decoder interface {
	io.Reader
	// decodeCrown tries to decode crown from incoming data.
	decodeCrown() (*gomel.Crown, error)
	// decodePreunit tries to decode preunit from incoming data.
	decodePreunit() (gomel.Preunit, error)
	// decodeChunk tries to a slice consecutive antichains from incoming data.
	decodeChunk() ([]gomel.Preunit, error)
}
