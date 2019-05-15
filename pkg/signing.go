package gomel

import "bytes"

// Signature of a unit.
type Signature []byte

// SigEq checks signatures' equality
func SigEq(s, r Signature) bool {
	return bytes.Equal(s, r)
}

// PublicKey used for signature checking.
type PublicKey interface {
	// Verify checks if a preunit has a correct signature.
	Verify(Preunit) bool
}

// PrivateKey used for signing units.
type PrivateKey interface {
	// Sign computes and returns a signature of a preunit.
	Sign(Preunit) Signature
}
