package gomel

// PosetFactory is an interface to create posets
type PosetFactory interface {
	// CreatePoset creates empty poset with given number of processes
	CreatePoset(nProcesses int) Poset
}
