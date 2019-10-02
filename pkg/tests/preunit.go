package tests

import (
	"bytes"
	"encoding/binary"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

type preunit struct {
	creator        uint16
	parentsHeights []int
	signature      gomel.Signature
	hash           gomel.Hash
	controlHash    gomel.Hash
	data           []byte
	rsData         []byte
}

// NewPreunit creates a preunit.
func NewPreunit(creator uint16, parents []*gomel.Hash, parentsHeights []int, data []byte, rsData []byte) gomel.Preunit {
	pu := &preunit{
		creator:        creator,
		parentsHeights: parentsHeights,
		controlHash:    *gomel.CombineHashes(parents),
		data:           data,
		signature:      make([]byte, 64),
		rsData:         rsData,
	}
	pu.computeHash()

	return pu
}

// RandomSourceData is the random source data embedded in this preunit.
func (pu *preunit) RandomSourceData() []byte {
	return pu.rsData
}

// ParentsHeights is the sequence of heights of parents.
func (pu *preunit) ParentsHeights() []int {
	return pu.parentsHeights
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

// SetSignature sets the signature of the preunit.
func (pu *preunit) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

// computeHash computes the preunit's hash value and saves it in the corresponding field.
func (pu *preunit) computeHash() {
	var data bytes.Buffer
	creatorBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(creatorBytes, pu.creator)
	data.Write(creatorBytes)
	data.Write(pu.data)
	data.Write(pu.rsData)
	data.Write(pu.controlHash[:])
	sha3.ShakeSum128(pu.hash[:], data.Bytes())
}
