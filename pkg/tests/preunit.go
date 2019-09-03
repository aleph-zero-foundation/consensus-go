package tests

import (
	"bytes"
	"encoding/binary"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

type preunit struct {
	creator   int
	parents   []*gomel.Hash
	signature gomel.Signature
	hash      gomel.Hash
	data      []byte
	rsData    []byte
}

// NewPreunit creates a preunit.
func NewPreunit(creator int, parents []*gomel.Hash, data []byte, rsData []byte) gomel.Preunit {
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
func (pu *preunit) Creator() int {
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

// Parents returns hashes of the preunit's parents.
func (pu *preunit) Parents() []*gomel.Hash {
	return pu.parents
}

// SetSignature sets the signature of the preunit.
func (pu *preunit) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

// computeHash computes the preunit's hash value and puts it in the corresponding field.
func (pu *preunit) computeHash() {
	var data bytes.Buffer
	creatorBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(creatorBytes, uint16(pu.creator))
	data.Write(creatorBytes)
	for _, p := range pu.parents {
		data.Write(p[:])
	}
	data.Write(pu.Data())
	data.Write(pu.RandomSourceData())
	sha3.ShakeSum128(pu.hash[:], data.Bytes())
}
