package gomel

// Orderer orders ordered orders into ordered order.
type Orderer interface {
	// AddPreunits sends to orderer preunits received from other committee member.
	AddPreunits(uint16, ...Preunit)
	// UnitsByID finds units with given IDs in Orderer.
	// Returns nil on the corresponding position if the requested unit is not present.
	// In case of forks returns all known units with a particular ID.
	UnitsByID(...uint64) []Unit
	// UnitsByHash finds units with given IDs in Orderer.
	// Returns nil on the corresponding position if the requested unit is not present.
	UnitsByHash(...*Hash) []Unit
	// MaxUnits returns maximal units per process for the given epoch. Returns nil if epoch not known.
	MaxUnits(EpochID) SlottedUnits
	// GetInfo returns DagInfo of the newest epoch.
	GetInfo() [2]*DagInfo
	// Delta returns all the units present in orderer that are above heights indicated by provided DagInfo.
	// That includes also all units from newer epochs.
	Delta([2]*DagInfo) []Unit
	// Start starts the orderer using provided Syncer and Alerter.
	Start(Syncer, Alerter)
	Stop()
}
