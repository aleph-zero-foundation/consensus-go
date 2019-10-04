package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// DagFactory is an interface to create dags.
type DagFactory interface {
	// CreateDag creates empty dag with a given configuration.
	CreateDag(dc config.Dag) gomel.Dag
}

type testDagFactory struct{}

// NewTestDagFactory returns a factory for creating test dags.
func NewTestDagFactory() DagFactory {
	return testDagFactory{}
}

func (testDagFactory) CreateDag(dagConfiguration config.Dag) gomel.Dag {
	return newDag(dagConfiguration)
}

// NewTestDagFactoryWithChecks returns a factory for creating test dags with basic compliance checks.
func NewTestDagFactoryWithChecks() DagFactory {
	return defaultChecksFactory{}
}

type defaultChecksFactory struct{}

func (defaultChecksFactory) CreateDag(dc config.Dag) gomel.Dag {
	dag := newDag(dc)
	return check.ForkerMuting(check.NoSelfForkingEvidence(check.ParentConsistency(check.BasicCompliance(dag))))
}
