package gomel

// DagConfig contains required information to create a dag
type DagConfig struct {
	Keys []PublicKey
}

// NProc returns the number of processes in a given dagConfiguration
func (pc DagConfig) NProc() int {
	return len(pc.Keys)
}

// DagFactory is an interface to create dags
type DagFactory interface {
	// CreateDag creates empty dag with a given configuration
	CreateDag(pc DagConfig) Dag
}
