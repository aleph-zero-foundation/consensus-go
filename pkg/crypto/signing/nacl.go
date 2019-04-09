package signing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"golang.org/x/crypto/nacl/sign"
)

type _publicKey *[32]byte

type _privateKey *[64]byte

type publicKey struct {
	pub _publicKey
}

type privateKey struct {
	priv _privateKey
}

func (pub *publicKey) Verify(pu gomel.Preunit) bool {
	msgSig := append(pu.Signature(), pu.Hash()[:]...)
	_, v := sign.Open(nil, msgSig, pub.pub)
	return v
}

// Signs a unit and returns only the signature
func (priv *privateKey) Sign(pu gomel.Preunit) gomel.Signature {
	return sign.Sign(nil, pu.Hash()[:], priv.priv)[:sign.Overhead]
}

// GenerateKeys procucess a pair of keys for signing units
func GenerateKeys() (PublicKey, PrivateKey, error) {
	_pub, _priv, err := sign.GenerateKey(nil)
	if err != nil {
		return nil, nil, err
	}
	pub := &publicKey{pub: _pub}
	priv := &privateKey{priv: _priv}

	return pub, priv, nil
}
