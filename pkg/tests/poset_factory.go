package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type testDagFactory struct{}

// NewTestDagFactory returns instation of testDagFactory --- factory creating test dags
func NewTestDagFactory() gomel.DagFactory {
	return testDagFactory{}
}

func (testDagFactory) CreateDag(dagConfiguration gomel.DagConfig) gomel.Dag {
	return newDag(dagConfiguration)
}
