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

func getPreunits(conn network.Connection) ([][]gomel.Preunit, error) {
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

func addAntichain(poset gomel.Poset, preunits []gomel.Preunit) error {
	var wg sync.WaitGroup
	// TODO: We only report one error, we might want to change it when we deal with Byzantine processes.
	var problem error
	for _, preunit := range preunits {
		wg.Add(1)
		poset.AddUnit(preunit, func(_ gomel.Preunit, _ gomel.Unit, err error) {
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

// addUnits adds the provided units to the poset, assuming they are divided into antichains as described in toLayers
func addUnits(poset gomel.Poset, preunits [][]gomel.Preunit) error {
	for _, pus := range preunits {
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
//
// The precise flow of this protocol follows:
/*		1. Receive a consistent snapshot of the other parties maximal units as a list of (hash, height) pairs.
		2. Compute a similar info for our poset.
		3. Send this info.
		4. Compute and send units that are predecessors of the received info and succesors of ours.
		5. Compute and send requests, containing hashes in the other party's info not recognized by us.
		6. Receive units complying with the above restrictions and the ones we requested.
		7. Receive requests as above. If they were empty proceed to 9.
		8. Add units that are requested and their predecessors down to the first we know they have, and send all the units.
		9. Add the received units to the poset.
*/
func (p In) Run(poset gomel.Poset, conn network.Connection) {
	defer conn.Close()
	theirPosetInfo, err := getPosetInfo(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	maxSnapshot := posetMaxSnapshot(poset)
	posetInfo := toPosetInfo(maxSnapshot)
	if err := sendPosetInfo(posetInfo, conn); err != nil {
		// TOOD: Error handling.
		return
	}
	units := unitsToSend(poset, maxSnapshot, theirPosetInfo, make([][]gomel.Hash, len(theirPosetInfo)))
	err = sendUnits(units, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	requests := requestsToSend(poset, theirPosetInfo, make([][]gomel.Preunit, len(theirPosetInfo)))
	err = sendRequests(requests, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	theirPreunitsReceived, err := getPreunits(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	theirRequests, err := getRequests(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	if nonempty(theirRequests) {
		units = unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
		err = sendUnits(units, conn)
		if err != nil {
			// TOOD: Error handling.
			return
		}
	}
	err = addUnits(poset, theirPreunitsReceived)
	if err != nil {
		// TOOD: Error handling.
		return
	}
}

// Run handles the outgoing connection using info from the poset.
// This version uses 3-exchange "pullpush" protocol: send heights, receive heights, units and requests, send units and requests.
// If we sent some requests there is a 4th exchange where we once again get units. This should only happen due to forks.
//
// The precise flow of this protocol follows:
/*		1. Get a consistent snapshot of our maximal units and convert it to a list of (hash, height) pairs.
			2. Send this info.
			3. Receive a similar info created by the other party.
			4. Receive units, that are predecessors of the received info and succesors of ours.
			5. Receive a list of requests, containing hashes in our info not recognized by the other party.
			6. Compute units to send complying with the above restrictions.
			7. Add units that are requested and their predecessors down to the first we know they have and send all of the above.
			8. Compute requests to send as above and send them. Treat the units we received as known.
			9. If the sent requests were nonempty, wait for more units. All the units are resend.
		10. Add the received units to the poset.
*/
func (p Out) Run(poset gomel.Poset, conn network.Connection) {
	defer conn.Close()
	maxSnapshot := posetMaxSnapshot(poset)
	posetInfo := toPosetInfo(maxSnapshot)
	if err := sendPosetInfo(posetInfo, conn); err != nil {
		// TOOD: Error handling.
		return
	}
	theirPosetInfo, err := getPosetInfo(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	theirPreunitsReceived, err := getPreunits(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	theirRequests, err := getRequests(conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	units := unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
	err = sendUnits(units, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	requests := requestsToSend(poset, theirPosetInfo, theirPreunitsReceived)
	err = sendRequests(requests, conn)
	if err != nil {
		// TOOD: Error handling.
		return
	}
	if nonempty(requests) {
		theirPreunitsReceived, err = getPreunits(conn)
		if err != nil {
			// TOOD: Error handling.
			return
		}
	}
	err = addUnits(poset, theirPreunitsReceived)
	if err != nil {
		// TOOD: Error handling.
		return
	}
}
