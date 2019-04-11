package signing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"golang.org/x/crypto/nacl/sign"
)

type publicKeyData *[32]byte

type privateKeyData *[64]byte

type publicKey struct {
	data publicKeyData
}

type privateKey struct {
	data privateKeyData
}

func (pub *publicKey) Verify(pu gomel.Preunit) bool {
	msgSig := append(pu.Signature(), pu.Hash()[:]...)
	_, v := sign.Open(nil, msgSig, pub.data)
	return v
}

// Signs a unit and returns only the signature
func (priv *privateKey) Sign(pu gomel.Preunit) gomel.Signature {
	return sign.Sign(nil, pu.Hash()[:], priv.data)[:sign.Overhead]
}

// GenerateKeys procucess a pair of keys for signing units
func GenerateKeys() (PublicKey, PrivateKey, error) {
	pubData, privData, err := sign.GenerateKey(nil)
	if err != nil {
		return nil, nil, err
	}

	pub := &publicKey{pubData}
	priv := &privateKey{privData}

	return pub, priv, nil
}
