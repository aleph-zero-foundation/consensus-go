package gomel

// PosetConfig contains required information to create a poset
type PosetConfig struct {
	Keys []PublicKey
}

// NProc returns the number of processes in a given posetConfiguration
func (pc PosetConfig) NProc() int {
	return len(pc.Keys)
}

// PosetFactory is an interface to create posets
type PosetFactory interface {
	// CreatePoset creates empty poset with a given configuration
	CreatePoset(pc PosetConfig) Poset
}
