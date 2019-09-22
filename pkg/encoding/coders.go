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
type Encoder interface {
	io.Writer
	// EncodeUnit encodes a single unit.
	EncodeUnit(gomel.Unit) error
	// EncodeUnits encodes a slice of units.
	EncodeUnits([]gomel.Unit) error
}

// Decoder extends io.Reader with ability to encode units.
type Decoder interface {
	io.Reader
	// DecodePreunit tries to decode preunit from incoming data.
	DecodePreunit() (gomel.Preunit, error)
	// DecodePreunits tries to a slice consecutive antichains from incoming data.
	DecodePreunits() ([][]gomel.Preunit, int, error)
}
