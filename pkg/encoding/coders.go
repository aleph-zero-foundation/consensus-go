package encoding

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Encoder encodes data for sending it over the network.
type Encoder interface {
	// EncodeUnit encodes a unit to be sent over the network.
	// The encoder writes the encoded data to a io.Writer it was instantiated with.
	EncodeUnit(gomel.Unit) error
}

// Decoder decodes data received from the network.
type Decoder interface {
	// DecodePreunit reads encoded data from a io.Reader it was instantiated with and
	// tries to decode it as a preunit.
	DecodePreunit() (gomel.Preunit, error)
}
