package encoding

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Encoder encodes different type of data and writes them to a io.Writer that it was instantiated with.
type Encoder interface {
	// EncodeUnits encodes a slice of units and writes the encoded data to the io.Writer.
	EncodeUnits([]gomel.Unit) error
}

// Decoder reads data from io.Reader it was instantiated with and returns a decoded slice of preunits.
type Decoder interface {
	// DecodePreunits reads encoded data from the io.Reader and tries to decode them
	// as a silce of preunits.
	DecodePreunits() ([]gomel.Preunit, error)
}
