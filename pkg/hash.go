package gomel

// A hash, usually used to identify units.
type Hash [64]byte

// Returns a shortened version of the hash for easy viewing. For now quite stupid, because this might contain broken chars.
func (h *Hash) Short() string {
	return string(h[:8])
}
