package tests

import (
	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type testPosetFactory struct{}

// NewTestPosetFactory returns instation of testPosetFactory --- factory creating test posets
func NewTestPosetFactory() gomel.PosetFactory {
	return testPosetFactory{}
}

func (testPosetFactory) CreatePoset(posetConfiguration gomel.PosetConfig) gomel.Poset {
	return newPoset(posetConfiguration)
}
