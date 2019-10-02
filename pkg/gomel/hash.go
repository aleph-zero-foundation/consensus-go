package gomel

import (
	"bytes"
	"encoding/base64"

	"golang.org/x/crypto/sha3"
)

// Hash is a type storing hash values, usually used to identify units.
type Hash [32]byte

// Short returns a shortened version of the hash for easy viewing.
func (h *Hash) Short() string {
	return base64.StdEncoding.EncodeToString(h[:8])
}

// LessThan checks if h is less than k in lexicographic order.
// This is used to create a linear order on hashes.
func (h *Hash) LessThan(k *Hash) bool {
	for i := 0; i < len(h); i++ {
		if h[i] < k[i] {
			return true
		} else if h[i] > k[i] {
			return false
		}
	}
	return false
}

// XOR returns xor of two hashes.
func XOR(h *Hash, k *Hash) *Hash {
	var result Hash
	for i := range result {
		result[i] = h[i] ^ k[i]
	}
	return &result
}

// XOREqual updates hash to be a xor with given argument.
func (h *Hash) XOREqual(k *Hash) {
	for i := range h {
		h[i] ^= k[i]
	}
}

// ZeroHash is a hash containing zeros at all 32 positions.
var ZeroHash Hash

// CombineHashes computes hash from sequence of hashes.
func CombineHashes(hashes []*Hash) *Hash {
	var (
		result Hash
		data   bytes.Buffer
	)
	for _, h := range hashes {
		if h != nil {
			data.Write(h[:])
		} else {
			data.Write(ZeroHash[:])
		}
	}
	sha3.ShakeSum128(result[:], data.Bytes())
	return &result
}
