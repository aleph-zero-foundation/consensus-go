package creating

import (
	"bytes"
	"encoding/binary"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"golang.org/x/crypto/sha3"
)

type preunit struct {
	creator   int
	parents   []gomel.Hash
	signature gomel.Signature
	hash      gomel.Hash
}

func newPreunit(creator int, parents []gomel.Hash) gomel.Preunit {
	pu := &preunit{
		creator: creator,
		parents: parents,
	}
	pu.computeHash()

	return pu
}

// Returns the creator of the unit.
func (pu *preunit) Creator() int {
	return pu.creator
}

// Signature returns preunit's signature
func (pu *preunit) Signature() gomel.Signature {
	return pu.signature
}

// Computes and returns the hash of this unit.
func (pu *preunit) Hash() *gomel.Hash {
	return &pu.hash
}

// Returns the hashes of the unit's parents.
func (pu *preunit) Parents() []gomel.Hash {
	return pu.parents
}

// SetSignature sets signature of the preunit
func (pu *preunit) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

func toBytes(data interface{}) []byte {
	var newData bytes.Buffer
	binary.Write(&newData, binary.LittleEndian, data)
	return newData.Bytes()
}

func (pu *preunit) computeHash() {
	var data bytes.Buffer
	data.Write(toBytes(pu.creator))
	data.Write(toBytes(pu.parents))
	sha3.ShakeSum256(pu.hash[:len(pu.hash)], data.Bytes())
}
