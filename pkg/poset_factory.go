package gomel

// PosetConfiguration contains required information to create a poset
// TODO: Think about what should be contained in a posetConfigration, and if we want to move it
// to the config package and separate the process config and the poset config
type PosetConfig struct {
	nProcesses int
}

// NProc returns the number of processes in a given posetConfiguration
func (pc PosetConfig) NProc() int {
	return pc.nProcesses
}

// NewPosetConfiguration returns the PosetConfiguration with a given number of processes
func NewPosetConfig(nProcesses int) PosetConfig {
	return PosetConfig{nProcesses: nProcesses}
}

// PosetFactory is an interface to create posets
type PosetFactory interface {
	// CreatePoset creates empty poset with a given configuration
	CreatePoset(pc PosetConfig) Poset
}
