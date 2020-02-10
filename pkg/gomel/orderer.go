package gomel

// Orderer orders ordered orders into ordered order.
type Orderer interface {
	// AddPreunits sends to orderer a slice of Preunits received from other committee members.
	AddPreunits([]Preunit)
	// GetUnits with given IDs.
	GetUnits([]uint64) []Unit
	// GetInfo returns DagInfo for the newest epoch.
	GetInfo() DagInfo
	// GetDelta returns all the units present in orderer that are above heights indicated by provided DagInfo.
	// That includes also all units from newer epochs.
	GetDelta(DagInfo) []Unit
}
