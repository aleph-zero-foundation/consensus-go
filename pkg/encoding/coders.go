package encoding

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Encoder encodes data for sending it over the network.
type Encoder interface {
	// EncodeUnits encodes layers of units to be sent over the newtork.
	// The layers are represented as a slice of slices of units.
	// The encoder writes the encoded data to a io.Writer it was instantiated with.
	EncodeUnits([][]gomel.Unit) error
}

// Decoder decodes data recieved from the network.
type Decoder interface {
	// DecodePreunits reads encoded data from a io.Reader it was instantiated with and
	// tries to decode them as a silce of slices of preunits.
	DecodePreunits() ([][]gomel.Preunit, error)
}
