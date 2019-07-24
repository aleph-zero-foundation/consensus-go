package gomel

// Callback is a generic function called during AddUnit on the Preunit that is being added, and the resulting Unit (if successful) or encountered error (if not).
type Callback func(Preunit, Unit, error)

// Dag is the main data structure of the Aleph consensus protocol. It is built of units partially ordered by "is-parent-of" relation.
type Dag interface {
	// AddUnits tries to transform a preunit to a corresponding unit and add it to the dag. After that, it calls the Callback.
	AddUnit(Preunit, RandomSource, Callback)
	// PrimeUnits returns all prime units on a given level of the dag.
	PrimeUnits(int) SlottedUnits
	// MaximalUnitsPerProcess returns a collection of units containing, for each process, all maximal units created by that process.
	MaximalUnitsPerProcess() SlottedUnits
	// Get returns the units associated with the given hashes, in the same order.
	// If no unit with a hash exists in the dag, the result will contain a nil at the position of the hash.
	Get([]*Hash) []Unit
	// IsQuorum checks if the given number of processes is enough to form a quroum.
	IsQuorum(number int) bool
	// NProc returns the number of processes that shares this dag.
	NProc() int
}

// MergeCallbacks combines two callbacks into one.
func MergeCallbacks(cb1, cb2 Callback) Callback {
	return func(pu Preunit, unit Unit, err error) {
		cb1(pu, unit, err)
		cb2(pu, unit, err)
	}
}

// NopCallback returns an empty Callback.
func NopCallback() Callback {
	return func(Preunit, Unit, error) {}
}
