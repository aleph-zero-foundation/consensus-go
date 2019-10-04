package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
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

// NewTestDagFactoryWithChecks returns a factory for creating test dags with basic compliance checks.
func NewTestDagFactoryWithChecks() gomel.DagFactory {
	return defaultChecksFactory{}
}

type defaultChecksFactory struct{}

func (defaultChecksFactory) CreateDag(dc gomel.DagConfig) gomel.Dag {
	dag := newDag(dc)
	return check.ForkerMuting(check.NoSelfForkingEvidence(check.ParentConsistency(check.BasicCompliance(dag))))
}
