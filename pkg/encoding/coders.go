package encoding

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

// Encoder encodes different type of date and writes them to a io.Writer that it was instantiated with
type Encoder interface {
	EncodePreunits([]gomel.Unit) error
}

// Decoder reads data from io.Reader it was instantiated with and returns a decoded slice of preunits
type Decoder interface {
	DecodePreunits() ([]gomel.Unit, error)
}
