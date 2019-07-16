package tcoin

import (
	"crypto/subtle"
	"math/big"

	"github.com/cloudflare/bn256"
)

type verificationKey struct {
	key *bn256.G2
}

type secretKey struct {
	key *big.Int
}

type signature []byte

var gen = new(bn256.G2).ScalarBaseMult(big.NewInt(int64(1)))

func (vk *verificationKey) verify(s signature, msg *big.Int) bool {
	sHash := new(bn256.G1)
	_, err := sHash.Unmarshal(s)
	if err != nil {
		return false
	}

	p1 := bn256.Pair(sHash, gen).Marshal()
	// hashing of the form msg => msg * gen is NOT secure
	p2 := bn256.Pair(new(bn256.G1).ScalarBaseMult(msg), vk.key).Marshal()

	return subtle.ConstantTimeCompare(p1, p2) == 1
}

func (sk *secretKey) sign(msg *big.Int) signature {
	// hashing of the form msg => msg * gen is NOT secure
	msgHash := new(bn256.G1).ScalarBaseMult(msg)
	sgn := new(bn256.G1).ScalarMult(msgHash, sk.key)
	return sgn.Marshal()
}
