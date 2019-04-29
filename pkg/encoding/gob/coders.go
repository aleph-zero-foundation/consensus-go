package gob

import (
	"encoding/gob"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/creating"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"io"
)

// A helper type for sending units over the network
type netunit struct {
	Creator   int
	Signature gomel.Signature
	Parents   []gomel.Hash
}

type encoder struct {
	engine *gob.Encoder
}

// NewEncoder creates a new encoding.Encoder that is threadsafe
func NewEncoder(w io.Writer) encoding.Encoder {
	return &encoder{gob.NewEncoder(w)}
}

// EncodeUnits encodes a slice of slices of units and writes the encoded data to the io.Writer.
func (e *encoder) EncodeUnits(units [][]gomel.Unit) error {
	netunits := make([][]netunit, len(units))
	for i, layer := range units {
		netunits[i] = make([]netunit, len(layer))

		for j, unit := range layer {
			parentHashes := make([]gomel.Hash, len(unit.Parents()))
			for j, parent := range unit.Parents() {
				parentHashes[j] = *parent.Hash()
			}
			netunits[i][j] = netunit{unit.Creator(), unit.Signature(), parentHashes}
		}
	}
	return e.engine.Encode(netunits)
}

type decoder struct {
	engine *gob.Decoder
}

// NewDecoder creates a new encoding.Decoder that is threadsafe
func NewDecoder(r io.Reader) encoding.Decoder {
	return &decoder{gob.NewDecoder(r)}
}

// DecodePreunits reads encoded data from the io.Reader and tries to decode them
// as a silce of slices of preunits.
func (d *decoder) DecodePreunits() ([][]gomel.Preunit, error) {
	netunits := make([][]netunit, 0)
	if err := d.engine.Decode(&netunits); err != nil {
		return nil, err
	}
	preunits := make([][]gomel.Preunit, len(netunits))
	for i, layer := range netunits {
		preunits[i] = make([]gomel.Preunit, len(layer))
		for j, netunit := range layer {
			preunits[i][j] = creating.NewPreunit(netunit.Creator, netunit.Parents)
			preunits[i][j].SetSignature(netunit.Signature)
		}
	}
	return preunits, nil
}
