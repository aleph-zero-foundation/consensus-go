package gomel

// Orderer orders ordered orders into ordered order.
type Orderer interface {
	// AddPreunits sends to orderer preunits received from other committee member.
	AddPreunits(uint16, ...Preunit) []error
	// UnitsByID finds units with given IDs in Orderer.
	// Returns nil on the corresponding position if the requested unit is not present.
	// In case of forks returns all known units with a particular ID.
	UnitsByID(...uint64) []Unit
	// AddPreunits sends to orderer preunits received from other committee member.
	AddPreunits(uint16, ...Preunit)
	// GetInfo returns DagInfo of the newest epoch.
	GetInfo() [2]*DagInfo
	// Delta returns all the units present in orderer that are above heights indicated by provided DagInfo.
	// That includes also all units from newer epochs.
	Delta([2]*DagInfo) []Unit
	// UnitsByHash finds units with given IDs in Orderer.
	// Returns nil on the corresponding position if the requested unit is not present.
	UnitsByHash(...*Hash) []Unit
	// MaxUnits returns maximal units per process for the given epoch. Returns nil if epoch not known.
	MaxUnits(EpochID) SlottedUnits
	// SetAlerter binds the given Alerter to this orderer
	SetAlerter(Alerter)
	// SetSyncer binds the given Syncer to this orderer
	SetSyncer(Syncer)
	// Start starts the orderer using provided RandomSourceFactory, Syncer, and Alerter.
	Start(RandomSourceFactory, Syncer, Alerter)
	Stop()
}
