package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// DagFactory is an interface to create dags.
type DagFactory interface {
	// CreateDag creates empty dag with a given configuration.
	CreateDag(nProc uint16) gomel.Dag
}

type testDagFactory struct{}

// NewTestDagFactory returns a factory for creating test dags.
func NewTestDagFactory() DagFactory {
	return testDagFactory{}
}

func (testDagFactory) CreateDag(nProc uint16) gomel.Dag {
	return newDag(nProc)
}

// NewTestDagFactoryWithChecks returns a factory for creating test dags with basic compliance checks.
func NewTestDagFactoryWithChecks() DagFactory {
	return defaultChecksFactory{}
}

type defaultChecksFactory struct{}

func (defaultChecksFactory) CreateDag(nProc uint16) gomel.Dag {
	return check.ForkerMuting(check.NoSelfForkingEvidence(check.ParentConsistency(check.BasicCompliance(newDag(nProc)))))
}
