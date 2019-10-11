// Package gomel defines all the interfaces representing basic components for executing the Aleph protocol.
//
// The main components defined in this package are:
//  1. The unit and preunit representing the information produced by a single process in a single round of the protocol.
//  2. The dag, containing all the units created by processes and representing the partial order between them.
//  3. The random source interacting with the dag to generate randomness needed for the protocol.
//  4. The linear ordering that uses the dag and random source to eventually output a linear ordering of all units.
package gomel

import (
	"errors"
)

// Dag is the main data structure of the Aleph consensus protocol. It is built of units partially ordered by "is-parent-of" relation.
type Dag interface {
	// Decode attempts to decode the given Preunit and return a Unit or an error, when that is impossible.
	// The resulting Unit is NOT inserted in the dag -- to do that one needs to Emplace it.
	Decode(Preunit) (Unit, error)
	// Check if the Unit satisfies the assumptions of the dag. Should be called before Emplace.
	Check(Unit) error
	// Emplace attempts to add the given Unit to the dag. It returns the unit that is included in the dag.
	Emplace(Unit) Unit
	// PrimeUnits returns all prime units on a given level of the dag.
	PrimeUnits(int) SlottedUnits
	// UnitsOnHeight returns all units on a given height of the dag.
	UnitsOnHeight(int) SlottedUnits
	// MaximalUnitsPerProcess returns a collection of units containing, for each process, all maximal units created by that process.
	MaximalUnitsPerProcess() SlottedUnits
	// Get returns the units associated with the given hashes, in the same order.
	// If no unit with a hash exists in the dag, the result will contain a nil at the position of the hash.
	Get([]*Hash) []Unit
	// IsQuorum checks if the given number of processes is enough to form a quorum.
	IsQuorum(number uint16) bool
	// NProc returns the number of processes that shares this dag.
	NProc() uint16
}

// IsQuorum checks if subsetSize forms a quorum amongst all nProcesses.
func IsQuorum(nProcesses, subsetSize uint16) bool {
	return 3*subsetSize >= 2*nProcesses
}

// GetByControlHash searches the dag for a sequence of NProc units
// created by different processes, such that their heights and a controlHash
// matches with the given arguments.
// If there is no valid sequence of units it returns an error.
// This implementation checks among all the possibilities between the forks,
// which might be a very expensive computation.
func GetByControlHash(dag Dag, crown *Crown) ([]Unit, error) {
	possibleUnits := make([][]Unit, dag.NProc())
	unknown := 0
	for i, h := range crown.Heights {
		if h == -1 {
			continue
		}
		su := dag.UnitsOnHeight(h)
		possibleUnits[i] = su.Get(uint16(i))
		if possibleUnits[i] == nil {
			unknown++
		}
	}
	if unknown > 0 {
		return nil, NewUnknownParents(unknown)
	}
	return getTraversal(possibleUnits, crown.Heights, &crown.ControlHash)
}

func getTraversal(units [][]Unit, heights []int, hash *Hash) ([]Unit, error) {
	nProc := len(units)
	answer := make([]Unit, nProc)
	hashes := make([]*Hash, nProc)
	var rec func(int) bool
	rec = func(ind int) bool {
		if ind == nProc {
			if *hash == *CombineHashes(hashes) {
				return true
			}
			return false
		}
		if heights[ind] == -1 {
			return rec(ind + 1)
		}
		for _, u := range units[ind] {
			answer[ind] = u
			hashes[ind] = u.Hash()
			if rec(ind + 1) {
				return true
			}
			hashes[ind] = nil
		}
		return false
	}
	if rec(0) {
		return answer, nil
	}
	return nil, errors.New("wrong control hash")
}
