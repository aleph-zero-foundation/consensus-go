package gomel

// Signature of a unit.
type Signature []byte

// SigEq checks signatures' equality
func SigEq(s, r Signature) bool {
	if len(s) != len(r) {
		return false
	}
	for i, ri := range r {
		if s[i] != ri {
			return false
		}
	}
	return true
}
