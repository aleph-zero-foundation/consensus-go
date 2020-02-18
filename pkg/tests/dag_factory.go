package tests

import (
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

// DagFactory is an interface to create dags.
type DagFactory interface {
	// CreateDag creates empty dag with a given configuration.
	CreateDag(nProc uint16) (gomel.Dag, gomel.Adder)
}

type testDagFactory struct {
	epochID gomel.EpochID
}

// NewTestDagFactory returns a factory for creating test dags.
func NewTestDagFactory() DagFactory {
	return testDagFactory{}
}

// NewTestDagFactoryWithEpochID returns a factory for creating test dags.
func NewTestDagFactoryWithEpochID(id gomel.EpochID) DagFactory {
	return testDagFactory{id}
}

func (tdf testDagFactory) CreateDag(nProc uint16) (gomel.Dag, gomel.Adder) {
	cnf := config.Empty()
	cnf.NProc = nProc
	dag := newDag(cnf, tdf.epochID)
	adder := NewAdder(dag)
	return dag, adder
}

// NewTestDagFactoryWithChecks returns a factory for creating test dags with basic compliance checks.
func NewTestDagFactoryWithChecks() DagFactory {
	return defaultChecksFactory{}
}

type defaultChecksFactory struct{}

func (defaultChecksFactory) CreateDag(nProc uint16) (gomel.Dag, gomel.Adder) {
	cnf := config.Empty()
	cnf.NProc = nProc
	dag := newDag(cnf, gomel.EpochID(0))
	dag.AddCheck(check.BasicCorrectness)
	dag.AddCheck(check.ParentConsistency)
	dag.AddCheck(check.NoSelfForkingEvidence)
	dag.AddCheck(check.ForkerMuting)
	adder := NewAdder(dag)
	return dag, adder
}
