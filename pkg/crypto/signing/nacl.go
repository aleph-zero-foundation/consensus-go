package signing

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"golang.org/x/crypto/nacl/sign"
)

type publicKeyData *[32]byte

type privateKeyData *[64]byte

// publicKey implements PublicKey interface using NaCl library.
type publicKey struct {
	data publicKeyData
}

// privateKey implements PrivateKey interface using NaCl library.
type privateKey struct {
	data privateKeyData
}

// Verify checks if a given preunit has the correct signature using the public key from the receiver.
func (pub *publicKey) Verify(pu gomel.Preunit) bool {
	msgSig := append(pu.Signature(), pu.Hash()[:]...)
	_, v := sign.Open(nil, msgSig, pub.data)
	return v
}

// Sign takes the hash of a given preunit and returns its signature produced using the private key from the receiver.
func (priv *privateKey) Sign(pu gomel.Preunit) gomel.Signature {
	return sign.Sign(nil, pu.Hash()[:], priv.data)[:sign.Overhead]
}

// GenerateKeys produces a public and private key pair for signing units.
func GenerateKeys() (gomel.PublicKey, gomel.PrivateKey, error) {
	pubData, privData, err := sign.GenerateKey(nil)
	if err != nil {
		return nil, nil, err
	}

	pub := &publicKey{pubData}
	priv := &privateKey{privData}

	return pub, priv, nil
}
