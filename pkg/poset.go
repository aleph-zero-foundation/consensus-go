package gomel

// A poset, as defined by the Aleph whitepaper.
type Poset interface {
	AddUnit(pu Preunit, callback func(Preunit, Unit, error))
	PrimeUnits(level int) SlottedUnits
	MaximalUnitsPerProcess() SlottedUnits
}
