package tests

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
	txs       []gomel.Tx
}

func newPreunit(creator int, parents []gomel.Hash, txs []gomel.Tx) gomel.Preunit {
	pu := &preunit{
		creator: creator,
		parents: parents,
		txs:     txs,
	}
	pu.computeHash()

	return pu
}

// Txs returns transactions embedded in this preunit.
func (pu *preunit) Txs() []gomel.Tx {
	return pu.txs
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
func (pu *preunit) Parents() []gomel.Hash {
	return pu.parents
}

// SetSignature sets signature of the preunit.
func (pu *preunit) SetSignature(sig gomel.Signature) {
	pu.signature = sig
}

// toBytes returns a byte representation of any object.
func toBytes(data interface{}) []byte {
	var newData bytes.Buffer
	binary.Write(&newData, binary.LittleEndian, data)
	return newData.Bytes()
}

// computeHash computes preunit's hash value and puts it in the corresponding field.
func (pu *preunit) computeHash() {
	var data bytes.Buffer
	data.Write(toBytes(int32(pu.creator)))
	data.Write(toBytes(pu.parents))
	sha3.ShakeSum256(pu.hash[:len(pu.hash)], data.Bytes())
}
