package gomel

// Poset is the main data structure of the Aleph consensus protocol. It is built of units partially ordered by "is-parent-of" relation.
type Poset interface {
	// AddUnits tries to transform a preunit to a corresponding unit and add it to the poset.
	// After that, the generic callback function is called on that preunit, unit and potential error raised during these operations.
	AddUnit(Preunit, func(Preunit, Unit, error))
	// PrimeUnits returns all prime units on a given level of the poset.
	PrimeUnits(int) SlottedUnits
	// MaximalUnitsPerProcess returns a collection of units containing, for each process, all maximal units created by that process.
	MaximalUnitsPerProcess() SlottedUnits
	// IsQuorum checks if the given number of processes is enough to form a quroum.
	IsQuorum(number int) bool
	// GetNProcesses returns the number of processes that shares this poset.
	GetNProcesses() int
}
