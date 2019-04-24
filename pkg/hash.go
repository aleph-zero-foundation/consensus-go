package gomel

// A hash, usually used to identify units.
type Hash [64]byte

// Returns a shortened version of the hash for easy viewing. For now quite stupid, because this might contain broken chars.
func (h *Hash) Short() string {
	return string(h[:8])
}

// Checks if a hash is less than another hash in lexicographic order
// This is used to create linear order on hashes
func (h *Hash) LessThan(k *Hash) bool {
	for i := 0; i < 64; i++ {
		if h[i] < k[i] {
			return true
		} else if h[i] > k[i] {
			return false
		}
	}
	return false
}
