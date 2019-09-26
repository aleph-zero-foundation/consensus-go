package tests

import (
	"bytes"
	"encoding/binary"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

type preunit struct {
	creator     uint16
	parents     []*gomel.Hash
	signature   gomel.Signature
	hash        gomel.Hash
	controlHash gomel.Hash
	data        []byte
	rsData      []byte
}

// NewPreunit creates a preunit.
func NewPreunit(creator uint16, parents []*gomel.Hash, data []byte, rsData []byte) gomel.Preunit {
	pu := &preunit{
		creator:   creator,
		parents:   parents,
		data:      data,
		signature: make([]byte, 64),
		rsData:    rsData,
	}
	pu.computeHash()

	return pu
}

func (pu *preunit) RandomSourceData() []byte {
	return pu.rsData
}

// Data returns data embedded in this preunit.
func (pu *preunit) Data() []byte {
	return pu.data
}

// Creator of the preunit.
func (pu *preunit) Creator() uint16 {
	return pu.creator
}

// Signature of the preunit.
func (pu *preunit) Signature() gomel.Signature {
	return pu.signature
}

// Hash of the preunit.
func (pu *preunit) Hash() *gomel.Hash {
	return &pu.hash
}

// ControlHash of the preunit.
func (pu *preunit) ControlHash() *gomel.Hash {
	return &pu.controlHash
}

// Parents returns hashes of the preunit's parents.
func (pu *preunit) Parents() []*gomel.Hash {
	return pu.parents
}

// SetSignature sets the signature of the preunit.
func (pu *preunit) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

// computeHash computes the preunit's hash value and saves it in the corresponding field.
func (pu *preunit) computeHash() {
	var data bytes.Buffer
	for _, p := range pu.parents {
		if p != nil {
			data.Write(p[:])
		} else {
			data.Write(gomel.ZeroHash[:])
		}
	}
	sha3.ShakeSum128(pu.controlHash[:], data.Bytes())

	creatorBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(creatorBytes, pu.creator)
	data.Write(creatorBytes)
	data.Write(pu.data)
	data.Write(pu.rsData)
	data.Write(pu.controlHash[:])
	sha3.ShakeSum128(pu.hash[:], data.Bytes())
}
