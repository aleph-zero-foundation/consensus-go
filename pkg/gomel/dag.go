// Package gomel defines all the interfaces representing basic components for executing the Aleph protocol.
//
// The main components defined in this package are:
//  1. The unit and preunit representing the information produced by a single process in a single round of the protocol.
//  2. The dag, containing all the units created by processes and representing the partial order between them.
//  3. The random source interacting with the dag to generate randomness needed for the protocol.
//  4. The linear ordering that uses the dag and random source to eventually output a linear ordering of all units.
package gomel

import "gitlab.com/alephledger/core-go/pkg/crypto"

// Dag is the main data structure of the Aleph consensus protocol. It is built of units partially ordered by "is-parent-of" relation.
type Dag interface {
	// EpochID is a unique identifier of the epoch for this dag instance.
	EpochID() EpochID
	// DecodeParents returns a slice of parents of the given preunit, if the control hash matches.
	DecodeParents(Preunit) ([]Unit, error)
	// BuildUnit constructs a new unit from the preunit and the slice of parents.
	BuildUnit(Preunit, []Unit) Unit
	// Check runs on the given unit a series of UnitChechers added to the dag with AddCheck.
	Check(Unit) error
	// Insert puts a unit into the dag.
	Insert(Unit)
	// PrimeUnits returns all prime units on a given level of the dag.
	PrimeUnits(int) SlottedUnits
	// UnitsOnHeight returns all units on a given height of the dag.
	UnitsOnHeight(int) SlottedUnits
	// MaximalUnitsPerProcess returns a collection of units containing, for each process, all maximal units created by that process.
	MaximalUnitsPerProcess() SlottedUnits
	// GetUnit returns a unit with the given hash, if present in the dag, or nil otherwise.
	GetUnit(*Hash) Unit
	// GetUnits returns slice of units associated with given hashes, in the same order.
	// If no unit with a particular hash exists in the dag, the result contains a nil at that position.
	GetUnits([]*Hash) []Unit
	// GetByID returns the units associated with the given ID. There will be more than one only in the case of forks.
	GetByID(uint64) []Unit
	// IsQuorum checks if the given number of processes is enough to form a quorum.
	IsQuorum(uint16) bool
	// NProc returns the number of processes that shares this dag.
	NProc() uint16
	// AddCheck extends the list of UnitCheckers that are used by this dag during adding a unit.
	AddCheck(UnitChecker)
	// BeforeInsert adds an action to perform before insert.
	BeforeInsert(InsertHook)
	// AfterInsert adds an action to perform after insert.
	AfterInsert(InsertHook)
}

// MinimalQuorum is the minimal possible size of a subset forming a quorum within nProcesses.
func MinimalQuorum(nProcesses uint16) uint16 {
	return crypto.MinimalQuorum(nProcesses)
}

// MinimalTrusted is the minimal size of a subset of nProcesses, that guarantees
// that the subset contains at least one honest process.
func MinimalTrusted(nProcesses uint16) uint16 {
	return crypto.MinimalTrusted(nProcesses)
}
