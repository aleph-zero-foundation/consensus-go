package gomel

import "bytes"

// Signature of a unit.
type Signature []byte

// SigEq checks signatures' equality
func SigEq(s, r Signature) bool {
	return bytes.Equal(s, r)
}
