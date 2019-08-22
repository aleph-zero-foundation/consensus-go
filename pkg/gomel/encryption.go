package gomel

import "bytes"

// Cipher represents encrypted data
type CipherText []byte

// CTEq checks ciphertexts' equality
func CTEq(c1, c2 CipherText) bool {
	return bytes.Equal(s, r)
}

// EncryptionKey is used for encrypting data
type EncryptionKey interface {
	// Encrypt encrypts data
	Encrypt([]byte) CipherText
	// Encode encodes the encryption key in base 64.
	Encode() string
}

// DecryptionKey is used for decrypting ciphertexts encrypted with corresponding encryption key
type DecryptionKey interface {
	// Decrypt decrypts ciphertext that was encrypted with corresponding encryption key
	Decrypt([]byte) CipherText
	// Encode encodes the decryption key in base 64.
	Encode() string
}
