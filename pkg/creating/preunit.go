package creating

import (
	"bytes"
	"encoding/binary"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

type preunit struct {
	creator        uint16
	signature      gomel.Signature
	hash           gomel.Hash
	controlHash    gomel.Hash
	parentsHeights []int
	data           []byte
	rsData         []byte
}

// NewPreunit constructs a a new preunit with given parents and creator id.
func NewPreunit(creator uint16, controlHash *gomel.Hash, parentsHeights []int, data []byte, rsData []byte) gomel.Preunit {
	pu := &preunit{
		creator:        creator,
		parentsHeights: parentsHeights,
		controlHash:    *controlHash,
		data:           data,
		rsData:         rsData,
	}
	pu.computeHash()
	return pu
}

// RandomSourceData embedded in the preunit.
func (pu *preunit) RandomSourceData() []byte {
	return pu.rsData
}

// Data embedded in the preunit.
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

// ParentsHeights is the sequence of heights of parents.
func (pu *preunit) ParentsHeights() []int {
	return pu.parentsHeights
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
