package gob

import (
	"encoding/gob"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"io"
)

type netunit struct {
	creator   int
	signature gomel.Signature
	parents   []gomel.Hash
}

type encoder struct {
	engine *gob.Encoder
}

// NewEncoder creates a new encoding.Encoder that is threadsafe
func NewEncoder(w io.Writer) encoding.Encoder {
	return &encoder{gob.NewEncoder(w)}
}

func (e *encoder) EncodeUnits(units []gomel.Unit) error {
	netunits := make([]netunit, len(units))
	for i, unit := range units {
		parentHashes := make([]gomel.Hash, len(unit.Parents()))
		for j, parent := range unit.Parents() {
			parentHashes[j] = parent.Hash()
		}
		netunits[i] = netunit{unit.creator, unit.signature, parentHashes}
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

func (d *decoder) DecodeUnits() ([]gomel.Preunit, error) {
	netunits := make([]netunit)
	if err := d.engine.Decode(&netunits); err != nil {
		return nil, err
	}
}
