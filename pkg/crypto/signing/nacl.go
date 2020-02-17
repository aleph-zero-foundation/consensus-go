// Package signing implements unit signing using the NaCl library.
package signing

import (
	"encoding/base64"
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"golang.org/x/crypto/nacl/sign"
)

type publicKeyData *[32]byte

type privateKeyData *[64]byte

// publicKey implements PublicKey interface using the NaCl library.
type publicKey struct {
	data publicKeyData
}

// privateKey implements PrivateKey interface using the NaCl library.
type privateKey struct {
	data privateKeyData
}

// Verify checks if a given preunit has the correct signature using the public key from the receiver.
func (pub *publicKey) Verify(pu gomel.Preunit) bool {
	msgSig := append(pu.Signature(), pu.Hash()[:]...)
	_, v := sign.Open(nil, msgSig, pub.data)
	return v
}

func (pub *publicKey) Encode() string {
	return base64.StdEncoding.EncodeToString(pub.data[:])
}

// Sign takes the hash of a given preunit and returns its signature produced using the private key from the receiver.
func (priv *privateKey) Sign(h *gomel.Hash) gomel.Signature {
	return sign.Sign(nil, h[:], priv.data)[:sign.Overhead]
}

func (priv *privateKey) Encode() string {
	return base64.StdEncoding.EncodeToString(priv.data[:])
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

// DecodePublicKey decodes a public key encoded as a base64 string.
func DecodePublicKey(enc string) (gomel.PublicKey, error) {
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, err
	}
	if len(data) != 32 {
		return nil, errors.New("bad encoded public key")
	}
	result := publicKey{&[32]byte{}}
	copy(result.data[:], data)
	return &result, nil
}

// DecodePrivateKey decodes a private key encoded as a base64 string.
func DecodePrivateKey(enc string) (gomel.PrivateKey, error) {
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, err
	}
	if len(data) != 64 {
		return nil, errors.New("bad encoded private key")
	}
	result := privateKey{&[64]byte{}}
	for i, b := range data {
		result.data[i] = b
	}
	return &result, nil
}
