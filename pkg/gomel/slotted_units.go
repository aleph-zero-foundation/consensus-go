package gomel

// SlottedUnits defines a container for storing slices of units and accessing them using their creator's id.
type SlottedUnits interface {
	// Get all units in this container created by the process with the given id.
	// Note that in the main implementation, for efficiency reasons,
	// MODIFYING THE RETURNED VALUE DIRECTLY RESULTS IN UNDEFINED BEHAVIOUR!
	// Please avoid doing that.
	Get(uint16) []Unit
	// Set replaces all units in this container created by the process with the given id with given units.
	Set(uint16, []Unit)
	// Iterate through all units in this container, in chunks corresponding to different creator ids, until the given function returns false.
	Iterate(func([]Unit) bool)
}
