package gomel

import "gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"

// Poset is the main data structure of the Aleph consensus protocol. It is built of units partially ordered by "is-parent-of" relation.
type Poset interface {
	// AddUnits tries to transform a preunit to a corresponding unit and add it to the poset.
	// After that, the generic callback function is called on that preunit, unit and potential error raised during these operations.
	AddUnit(Preunit, func(Preunit, Unit, error))
	// PrimeUnits returns all prime units on a given level of the poset.
	PrimeUnits(int) SlottedUnits
	// MaximalUnitsPerProcess returns a collection of units containing, for each process, all maximal units created by that process.
	MaximalUnitsPerProcess() SlottedUnits
	// Get returns the units associated with the given hashes, in the same order.
	// If no unit with a hash exists in the poset, the result will contain a nil at the position of the hash.
	Get([]Hash) []Unit
	// IsQuorum checks if the given number of processes is enough to form a quroum.
	IsQuorum(number int) bool
	// NProc returns the number of processes that shares this poset.
	NProc() int
	// GetCRP returns common random permutation on a given level
	GetCRP(int) []int
	// ThresholdCoin returns threshold coin dealt by dealing unit having given hash
	// nil for hashes of non-dealing units
	ThresholdCoin(*Hash) *tcoin.ThresholdCoin
	// AddThresholdCoin adds thresholdCoin for given unit hash
	AddThresholdCoin(*Hash, *tcoin.ThresholdCoin)
	// RemoveThresholdCoin removes thresholdCoin for given unit hash
	RemoveThresholdCoin(*Hash)
}
