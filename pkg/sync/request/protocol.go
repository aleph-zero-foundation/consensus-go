package request

import (
	"encoding/gob"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	ggob "gitlab.com/alephledger/consensus-go/pkg/encoding/gob"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// In implements the side of the protocol that handles incoming connections.
type In struct{}

// Out implements the side of the protocol that handles outgoing connections.
type Out struct{}

func sendPosetInfo(info [][]*unitInfo, conn network.Connection) error {
	// TODO: We probably want a proper format, so one can write clients in other languages.
	return gob.NewEncoder(conn).Encode(info)
}

func getPosetInfo(conn network.Connection) ([][]*unitInfo, error) {
	info := [][]*unitInfo{}
	// TODO: We probably want a proper format, so one can write clients in other languages.
	err := gob.NewDecoder(conn).Decode(&info)
	return info, err
}

func sendUnits(units [][]gomel.Unit, conn network.Connection) error {
	return ggob.NewEncoder(conn).EncodeUnits(units)
}

func getUnits(conn network.Connection) ([][]gomel.Preunit, error) {
	return ggob.NewDecoder(conn).DecodePreunits()
}

func sendRequests(requests [][]gomel.Hash, conn network.Connection) error {
	// TODO: We probably want a proper format, so one can write clients in other languages.
	return gob.NewEncoder(conn).Encode(requests)
}

func getRequests(conn network.Connection) ([][]gomel.Hash, error) {
	requests := [][]gomel.Hash{}
	// TODO: We probably want a proper format, so one can write clients in other languages.
	err := gob.NewDecoder(conn).Decode(&requests)
	return requests, err
}

func addAntichain(poset gomel.Poset, units []gomel.Preunit) error {
	var wg sync.WaitGroup
	var problem error
	for _, unit := range units {
		wg.Add(1)
		poset.AddUnit(unit, func(_ gomel.Preunit, _ gomel.Unit, err error) {
			if err != nil {
				if _, ok := err.(*gomel.DuplicateUnit); !ok {
					// An error occurred that is not just attempting to add the same unit again.
					problem = err
				}
			}
			wg.Done()
		})
	}
	wg.Wait()
	return problem
}

func addUnits(poset gomel.Poset, units [][]gomel.Preunit) error {
	for _, pus := range units {
		err := addAntichain(poset, pus)
		if err != nil {
			return err
		}
	}
	return nil
}

func nonempty(requests [][]gomel.Hash) bool {
	for _, r := range requests {
		if len(r) > 0 {
			return true
		}
	}
	return false
}

// Run handles the incoming connection using info from the poset.
// This version uses 3-exchange "pullpush" protocol: receive heights, send heights, units and requests, receive units and requests.
// If we receive some requests there is a 4th exchange where we once again send units. This should only happen due to forks.
func (p In) Run(poset gomel.Poset, conn network.Connection) {
	defer conn.Close()
	// Receive info.
	theirPosetInfo, err := getPosetInfo(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	// Get a consistent snapshot of our maximal units and convert it to info.
	maxSnapshot := posetMaxSnapshot(poset)
	posetInfo := toPosetInfo(maxSnapshot)
	// Send the info.
	if err := sendPosetInfo(posetInfo, conn); err != nil {
		// TOOD: Error handling.
		return
	}
	// Compute and send units.
	units := unitsToSend(poset, maxSnapshot, theirPosetInfo, make([][]gomel.Hash, len(theirPosetInfo)))
	err = sendUnits(units, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	// Compute and send requests.
	requests, _ := requestsToSend(poset, theirPosetInfo, make([][]gomel.Preunit, len(theirPosetInfo)))
	err = sendRequests(requests, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	// Receive units and requests
	theirUnitsReceived, err := getUnits(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	theirRequests, err := getRequests(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	// If any were requested send more units.
	if nonempty(theirRequests) {
		units = unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
		err = sendUnits(units, conn)
		if err != nil {
			// TOOD: Error handling.
			return
		}
	}
	err = addUnits(poset, theirUnitsReceived)
	if err != nil {
		// TOOD: Error handling.
		return
	}
}

// Run handles the outgoing connection using info from the poset.
// This version uses 3-exchange "pullpush" protocol: send heights, receive heights, units and requests, send units and requests.
// If we sent some requests there is a 4th exchange where we once again get units. This should only happen due to forks.
func (p Out) Run(poset gomel.Poset, conn network.Connection) {
	defer conn.Close()
	// Get a consistent snapshot of our maximal units and convert it to info.
	maxSnapshot := posetMaxSnapshot(poset)
	posetInfo := toPosetInfo(maxSnapshot)
	// Send the info.
	if err := sendPosetInfo(posetInfo, conn); err != nil {
		// TOOD: Error handling.
		return
	}
	// Receive info, units, and requests
	theirPosetInfo, err := getPosetInfo(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	theirUnitsReceived, err := getUnits(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	theirRequests, err := getRequests(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	// Compute and send units.
	units := unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
	err = sendUnits(units, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	// Compute and send requests.
	requests, any := requestsToSend(poset, theirPosetInfo, theirUnitsReceived)
	err = sendRequests(requests, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	// If any were requested wait for more units.
	if any {
		theirUnitsReceived, err = getUnits(conn)
		if err != nil {
			// TOOD: Error handling.
			return
		}
	}
	err = addUnits(poset, theirUnitsReceived)
	if err != nil {
		// TOOD: Error handling.
		return
	}
}
