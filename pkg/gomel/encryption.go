package gomel

import "bytes"

// Cipher represents encrypted data
type CipherText []byte

// CTEq checks ciphertexts' equality
func CTEq(c1, c2 CipherText) bool {
	return bytes.Equal(s, r)
}

// EncryptionKey is used for encrypting messages 
type EncryptionKey interface {
	// Encrypt encrypts message
	Encrypt([]byte) (CipherText, error)
	// Encode encodes the encryption key in base 64.
	Encode() string
}

// DecryptionKey is used for decrypting ciphertexts encrypted with corresponding encryption key
type DecryptionKey interface {
	// Decrypt decrypts ciphertext that was encrypted with corresponding encryption key
	Decrypt(CipherText) ([]byte, error)
	// Encode encodes the decryption key in base 64.
	Encode() string
}
