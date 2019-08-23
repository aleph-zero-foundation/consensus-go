package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type testDagFactory struct{}

// NewTestDagFactory returns a factory for creating test dags.
func NewTestDagFactory() gomel.DagFactory {
	return testDagFactory{}
}

func (testDagFactory) CreateDag(dagConfiguration gomel.DagConfig) gomel.Dag {
	return newDag(dagConfiguration)
}
