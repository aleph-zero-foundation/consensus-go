package gomel

import "encoding/base64"

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
