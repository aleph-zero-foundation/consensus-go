package tests

import "gitlab.com/alephledger/consensus-go/pkg/gomel"

type orderer struct {
}

// NewOrderer returns an instance of the gomel.Orderer interface that is not performing any action when invoked.
// It is mainly used as a placeholder for the gomel.Orderer interface. In order to provide some non-trivial functionality,
// one should "override" some of its methods.
func NewOrderer() gomel.Orderer {
	return &orderer{}
}

func (o orderer) UnitsByID(...uint64) []gomel.Unit {
	return nil
}

func (o orderer) AddPreunits(uint16, ...gomel.Preunit) {
}

func (o orderer) GetInfo() [2]*gomel.DagInfo {
	return [2]*gomel.DagInfo{}
}

func (o orderer) Delta([2]*gomel.DagInfo) []gomel.Unit {
	return nil
}

func (o orderer) UnitsByHash(...*gomel.Hash) []gomel.Unit {
	return nil
}

func (o orderer) MaxUnits(gomel.EpochID) gomel.SlottedUnits {
	return nil
}

func (o orderer) SetAlerter(gomel.Alerter) {
}

func (o orderer) SetSyncer(gomel.Syncer) {
}

func (o orderer) Start(gomel.RandomSourceFactory, gomel.Syncer, gomel.Alerter) {
}

func (o orderer) Stop() {
}
