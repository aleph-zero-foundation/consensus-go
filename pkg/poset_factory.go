package gomel

// PosetConfiguration contains required information to create a poset
// TODO: Think about what should be contained in a posetConfigration, and if we want to move it
// to the config package and separate the process config and the poset config
type PosetConfiguration struct {
	nProcesses int
}

// GetNProcesses returns the number of processes in a given posetConfiguration
func (posetConfiguration PosetConfiguration) GetNProcesses() int {
	return posetConfiguration.nProcesses
}

// NewPosetConfiguration returns the PosetConfiguration with a given number of processes
func NewPosetConfiguration(nProcesses int) PosetConfiguration {
	return PosetConfiguration{nProcesses: nProcesses}
}

// PosetFactory is an interface to create posets
type PosetFactory interface {
	// CreatePoset creates empty poset with a given configuration
	CreatePoset(posetConfiguration PosetConfiguration) Poset
}
