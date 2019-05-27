package tcoin

import (
	"crypto/subtle"
	"math/big"

	"golang.org/x/crypto/bn256"
)

type verificationKey struct {
	key *bn256.G2
}

type secretKey struct {
	key *big.Int
}

// Signature is a signature
type Signature []byte

var gen = new(bn256.G2).ScalarBaseMult(big.NewInt(int64(1)))

// Verify verifies the signature
func (vk *verificationKey) verify(s Signature, msg *big.Int) bool {
	sHash, ok := new(bn256.G1).Unmarshal(s)
	if !ok {
		return false
	}

	p1 := bn256.Pair(sHash, gen).Marshal()
	p2 := bn256.Pair(new(bn256.G1).ScalarBaseMult(msg), vk.key).Marshal()

	return subtle.ConstantTimeCompare(p1, p2) == 1
}

// Sign signs the msg
func (sk *secretKey) sign(msg *big.Int) Signature {
	msgHash := new(bn256.G1).ScalarBaseMult(msg)
	sgn := new(bn256.G1).ScalarMult(msgHash, sk.key)
	return sgn.Marshal()
}
