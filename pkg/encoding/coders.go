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

// Encoder encodes data for sending it over the network.
type Encoder interface {
	// EncodeUnit encodes a unit to be sent over the network.
	// The encoder writes the encoded data to a io.Writer it was instantiated with.
	io.Writer
	EncodeUnit(gomel.Unit) error
	EncodeUnits([]gomel.Unit) error
}

// Decoder decodes data received from the network.
type Decoder interface {
	// DecodePreunit reads encoded data from a io.Reader it was instantiated with and
	// tries to decode it as a preunit.
	io.Reader
	DecodePreunit() (gomel.Preunit, error)
	DecodePreunits() ([][]gomel.Preunit, int, error)
}
