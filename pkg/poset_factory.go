package gomel

// DagConfig contains required information to create a dag
type DagConfig struct {
	Keys []PublicKey
}

// NProc returns the number of processes in a given dagConfiguration
func (dc DagConfig) NProc() int {
	return len(dc.Keys)
}

// DagFactory is an interface to create dags
type DagFactory interface {
	// CreateDag creates empty dag with a given configuration
	CreateDag(dc DagConfig) Dag
}
