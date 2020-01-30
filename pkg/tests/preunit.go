package tests

import (
	"bytes"
	"encoding/binary"
	"math"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/sha3"
)

type preunit struct {
	creator        uint16
	epochID        gomel.EpochID
	parentsHeights []int
	signature      gomel.Signature
	crown          gomel.Crown
	hash           gomel.Hash
	data           gomel.Data
	rsData         []byte
}

// NewPreunit creates a preunit.
func NewPreunit(creator uint16, crown *gomel.Crown, data gomel.Data, rsData []byte) gomel.Preunit {
	pu := &preunit{
		creator:   creator,
		crown:     *crown,
		data:      data,
		signature: make([]byte, 64),
		rsData:    rsData,
	}
	pu.computeHash()

	return pu
}

func (pu *preunit) EpochID() gomel.EpochID {
	return pu.epochID
}

// RandomSourceData is the random source data embedded in this preunit.
func (pu *preunit) RandomSourceData() []byte {
	return pu.rsData
}

// Data returns data embedded in this preunit.
func (pu *preunit) Data() gomel.Data {
	return pu.data
}

// Creator of the preunit.
func (pu *preunit) Creator() uint16 {
	return pu.creator
}

// Height of the preunit.
func (pu *preunit) Height() int {
	return pu.crown.Heights[pu.creator] + 1
}

// Signature of the preunit.
func (pu *preunit) Signature() gomel.Signature {
	return pu.signature
}

// View returns crown consisting all the parents of the unit.
func (pu *preunit) View() *gomel.Crown {
	return &pu.crown
}

// Hash of the preunit.
func (pu *preunit) Hash() *gomel.Hash {
	return &pu.hash
}

// SetSignature sets the signature of the preunit.
func (pu *preunit) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

// computeHash computes the preunit's hash value and saves it in the corresponding field.
func (pu *preunit) computeHash() {
	var data bytes.Buffer
	idBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(idBytes, gomel.UnitID(pu))
	data.Write(idBytes)
	data.Write(pu.data)
	data.Write(pu.rsData)
	heightBytes := make([]byte, 4)
	for _, h := range pu.crown.Heights {
		if h == -1 {
			binary.LittleEndian.PutUint32(heightBytes, math.MaxUint32)
		} else {
			binary.LittleEndian.PutUint32(heightBytes, uint32(h))
		}
		data.Write(heightBytes)
	}
	data.Write(pu.crown.ControlHash[:])
	sha3.ShakeSum128(pu.hash[:], data.Bytes())
}
