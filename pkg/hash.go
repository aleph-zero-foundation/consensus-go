package gomel

// Hash is a type storing hash values, usually used to identify units.
type Hash [64]byte

// Short returns a shortened version of the hash for easy viewing.
// For now quite stupid, might contain broken chars.
func (h *Hash) Short() string {
	return string(h[:8])
}

// LessThan checks if h is less than k in a lexicographic order.
// This is used to create linear order on hashes
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
