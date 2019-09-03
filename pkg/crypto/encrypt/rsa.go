package encrypt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"math/big"
	"strconv"
	"strings"

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
func GenerateKeys() (EncryptionKey, DecryptionKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	return &encryptionKey{&privKey.PublicKey}, &decryptionKey{privKey}, nil
}

func (ek *encryptionKey) Encrypt(msg []byte) (CipherText, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, ek.encKey, msg, nil)
}

func (dk *decryptionKey) Decrypt(ct CipherText) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, dk.decKey, ct, nil)
}

func (ek *encryptionKey) Encode() string {
	return ek.encKey.N.Text(big.MaxBase) + "|" + strconv.Itoa(ek.encKey.E)
}

// NewEncryptionKey creates encryptionKey from string representation
func NewEncryptionKey(text string) (EncryptionKey, error) {
	msg := "wrong format of encryption key"
	data := strings.Split(text, "|")
	if len(data) != 2 {
		return nil, gomel.NewDataError(msg)
	}
	N, ok := new(big.Int).SetString(data[0], big.MaxBase)
	if !ok {
		return nil, gomel.NewDataError(msg)
	}
	if N.Sign() != 1 {
		return nil, gomel.NewDataError(msg)
	}
	E, err := strconv.Atoi(data[1])
	if err != nil {
		return nil, err
	}
	return &encryptionKey{&rsa.PublicKey{N, E}}, nil
}
