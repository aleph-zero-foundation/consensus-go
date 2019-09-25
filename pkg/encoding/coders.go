// Package encoding implements custom encoding of units.
//
// Note that the objects being encoded are units, while the objects resulting from decoding are preunits.
// This makes perfect sense, as we only want to send information about units we already added to our dag,
// and any decoded information will have to be manually included into the dag.
//
// Crucially, and in contrast to Gob, this encoding only reads the bytes it needs.
package encoding

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"io"
)

// Encoder extends io.Writer with ability to encode units.
type encoder interface {
	io.Writer
	// encodeUnit encodes a single unit.
	encodeUnit(gomel.Unit) error
	// encodeAntichain encodes a single antichain of units.
	encodeAntichain([]gomel.Unit) error
	// encodeChunk encodes a slice of units by splitting them into antichains.
	encodeChunk([]gomel.Unit) error
}

// Decoder extends io.Reader with ability to encode units.
type decoder interface {
	io.Reader
	// decodePreunit tries to decode preunit from incoming data.
	decodePreunit() (gomel.Preunit, error)
	// decodeAntichain tries to decode an antichain of preunits from incoming data.
	decodeAntichain() ([]gomel.Preunit, error)
	// decodeChunk tries to a slice consecutive antichains from incoming data.
	decodeChunk() ([][]gomel.Preunit, int, error)
}
