package encrypt

import (
    "crypto/rand"
	"crypto/rsa"
    "crypto/sha256"
	"encoding/base64"
	"errors"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// encryptionKey implements EncryptionKey interface using stdlib crypto/rsa
type encryptionKey struct {
	encKey *rsa.PublicKey
}

// decryptionKey implements DecryptionKey interface using stdlib crypto/rsa
type decryptionKey struct {
	decKey *rsa.PrivateKey
}

// GenerateKeys creates a pair of keys for encryption/decryption
func GenerateKeys () (gomel.EncryptionKey, gomel.DecryptionKey, error) {
    privKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, nil, err
    }
    return encKey{&privKey.PublicKey}, decKey{&privKey}, nil
}

func (ek *encryptionKey) Encrypt(msg []byte) (gomel.CipherText, error) {
    return rsa.EncryptOAEP(sha256.New(), rand.Reader, ek.encKey, msg, nil)
}

func (ek *encryptionKey) Encode() []byte {
}

func (dk *decryptionKey) Decrypt(ct gomel.CipherText) ([]byte, error) {
    return rsa.DecryptOAEP(sha256.New(), rand.Reader, dk.decKey, ct, nil)
}

func (dk *decryptionKey) Encode() []byte {
}
