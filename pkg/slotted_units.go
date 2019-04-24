package gomel

// SlottedUnits interface defines a container for storing slices of units and access them using their creator's id.
type SlottedUnits interface {
	// Get all units in this container created by the process with the given id.
	Get(int) []Unit
	// Set replaces all units in this container created by the process with the given id with given units.
	Set(int, []Unit)
	// Iterate through all units in this container, in chunks corresponding to different creator ids, until the given function return false
	Iterate(func([]Unit) bool)
}
