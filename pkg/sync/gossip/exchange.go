package gossip

import (
	"sync"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/crypto/tcoin"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

func sendPosetInfo(info posetInfo, conn network.Connection) error {
	for _, pi := range info {
		err := encodeProcessInfo(conn, &pi)
		if err != nil {
			return err
		}
	}
	return conn.Flush()
}

func getPosetInfo(nProc int, conn network.Connection) (posetInfo, error) {
	info := make(posetInfo, nProc)
	for i := range info {
		pi, err := decodeProcessInfo(conn)
		if err != nil {
			return nil, err
		}
		info[i] = *pi
	}
	return info, nil
}

func sendUnits(units []gomel.Unit, conn network.Connection) error {
	err := encodeUnits(conn, toLayers(units))
	if err != nil {
		return err
	}
	return conn.Flush()
}

func getPreunits(conn network.Connection) ([][]gomel.Preunit, int, error) {
	return decodeUnits(conn)
}

func sendRequests(req requests, theirPosetInfo posetInfo, conn network.Connection) error {
	err := encodeRequests(conn, &req, theirPosetInfo)
	if err != nil {
		return err
	}
	return conn.Flush()
}

func getRequests(nProc int, myPosetInfo posetInfo, conn network.Connection) (requests, error) {
	result, err := decodeRequests(conn, myPosetInfo)
	return *result, err
}

func addAntichain(poset gomel.Poset, preunits []gomel.Preunit, myPid uint16, log zerolog.Logger) (bool, error) {
	var wg sync.WaitGroup
	// TODO: We only report one error, we might want to change it when we deal with Byzantine processes.
	var problem error
	primeAdded := false
	for _, preunit := range preunits {
		if len(preunit.Parents()) == 0 {
			tc, err := tcoin.Decode(preunit.ThresholdCoinData(), int(myPid))
			if err != nil {
				problem = err
				break
			}
			poset.AddThresholdCoin(preunit.Hash(), tc)
		}
		wg.Add(1)
		poset.AddUnit(preunit, func(pu gomel.Preunit, added gomel.Unit, err error) {
			if err != nil {
				switch e := err.(type) {
				case *gomel.DuplicateUnit:
					log.Info().Int(logging.Creator, e.Unit.Creator()).Int(logging.Height, e.Unit.Height()).Msg(logging.DuplicatedUnit)
				default:
					if len(pu.Parents()) == 0 {
						poset.RemoveThresholdCoin(pu.Hash())
					}
					problem = err
				}
			} else {
				if gomel.Prime(added) {
					primeAdded = true
				}
			}
			wg.Done()
		})
	}
	wg.Wait()
	return primeAdded, problem
}

// addUnits adds the provided units to the poset, assuming they are divided into antichains as described in toLayers
func addUnits(poset gomel.Poset, preunits [][]gomel.Preunit, myPid uint16, attemptTiming chan<- int, log zerolog.Logger) error {
	for _, pus := range preunits {
		primeAdded, err := addAntichain(poset, pus, myPid, log)
		if err != nil {
			return err
		}
		if primeAdded {
			select {
			case attemptTiming <- -1:
			default:
			}
		}
	}
	return nil
}

func nonempty(req requests) bool {
	for _, r := range req {
		if len(r) > 0 {
			return true
		}
	}
	return false
}

// inExchange handles the incoming connection using info from the poset.
// This version uses 3-exchange "pullpush" protocol: receive heights, send heights, units and requests, receive units and requests.
// If we receive some requests there is a 4th exchange where we once again send units. This should only happen due to forks.
//
// The precise flow of this protocol follows:
/*		1. Receive a consistent snapshot of the other parties maximal units as a list of (hash, height) pairs.
		2. Compute a similar info for our poset.
		3. Send this info.
		4. Compute and send units that are predecessors of the received info and successors of ours.
		5. Compute and send requests, containing hashes in the other party's info not recognized by us.
		6. Receive units complying with the above restrictions and the ones we requested.
		7. Receive requests as above. If they were empty proceed to 9.
		8. Add units that are requested and their predecessors down to the first we know they have, and send all the units.
		9. Add the received units to the poset.
*/
func inExchange(pid uint16, poset gomel.Poset, attemptTiming chan<- int, conn network.Connection) {
	log := conn.Log()
	log.Info().Msg(logging.SyncStarted)
	nProc := poset.NProc()

	log.Debug().Msg(logging.GetPosetInfo)
	theirPosetInfo, err := getPosetInfo(nProc, conn)
	if err != nil {
		log.Error().Str("where", "proto.In.getPosetInfo").Msg(err.Error())
		return
	}

	maxSnapshot := posetMaxSnapshot(poset)
	posetInfo := toPosetInfo(maxSnapshot)
	log.Debug().Msg(logging.SendPosetInfo)
	if err := sendPosetInfo(posetInfo, conn); err != nil {
		log.Error().Str("where", "proto.In.sendPosetInfo").Msg(err.Error())
		return
	}

	units, err := unitsToSend(poset, maxSnapshot, theirPosetInfo, nil)
	if err != nil {
		log.Error().Str("where", "proto.In.unitsToSend").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.SendUnits)
	err = sendUnits(units, conn)
	if err != nil {
		log.Error().Str("where", "proto.In.sendUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(units)).Msg(logging.SentUnits)

	req := requestsToSend(poset, theirPosetInfo, newStaticHashSet(nil))
	log.Debug().Msg(logging.SendRequests)
	err = sendRequests(req, theirPosetInfo, conn)
	if err != nil {
		log.Error().Str("where", "proto.In.sendRequests").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.GetPreunits)
	theirPreunitsReceived, nReceived, err := getPreunits(conn)
	if err != nil {
		log.Error().Str("where", "proto.In.getPreunits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nReceived).Msg(logging.ReceivedPreunits)

	log.Debug().Msg(logging.GetPreunits)
	theirFreshPreunitsReceived, nFreshReceived, err := getPreunits(conn)
	if err != nil {
		log.Error().Str("where", "proto.In.getPreunits fresh").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nFreshReceived).Msg(logging.ReceivedPreunits)

	log.Debug().Msg(logging.GetRequests)
	theirRequests, err := getRequests(nProc, posetInfo, conn)
	if err != nil {
		log.Error().Str("where", "proto.In.getRequests").Msg(err.Error())
		return
	}

	if nonempty(theirRequests) {
		log.Info().Msg(logging.AdditionalExchange)
		units, err = unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
		if err != nil {
			log.Error().Str("where", "proto.In.unitsToSend(extra round)").Msg(err.Error())
			return
		}
		log.Debug().Msg(logging.SendUnits)
		err = sendUnits(units, conn)
		if err != nil {
			log.Error().Str("where", "proto.In.sendUnits(extra round)").Msg(err.Error())
			return
		}
		log.Debug().Int(logging.Size, len(units)).Msg(logging.SentUnits)
	}

	log.Debug().Msg(logging.AddUnits)
	err = addUnits(poset, theirPreunitsReceived, pid, attemptTiming, log)
	if err != nil {
		log.Error().Str("where", "proto.In.addUnits").Msg(err.Error())
		return
	}
	err = addUnits(poset, theirFreshPreunitsReceived, p.MyPid, p.AttemptTiming, log)
	if err != nil {
		log.Error().Str("where", "proto.In.addUnits fresh").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Sent, len(units)).Int(logging.Recv, nReceived).Msg(logging.SyncCompleted)
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
func outExchange(pid uint16, poset gomel.Poset, attemptTiming chan<- int, conn network.Connection) {
	log := conn.Log()
	log.Info().Msg(logging.SyncStarted)
	nProc := poset.NProc()

	maxSnapshot := posetMaxSnapshot(poset)
	posetInfo := toPosetInfo(maxSnapshot)
	log.Debug().Msg(logging.SendPosetInfo)
	if err := sendPosetInfo(posetInfo, conn); err != nil {
		log.Error().Str("where", "proto.Out.sendPosetInfo").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.GetPosetInfo)
	theirPosetInfo, err := getPosetInfo(nProc, conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.getPosetInfo").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.GetPreunits)
	theirPreunitsReceived, nReceived, err := getPreunits(conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.getPreunits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nReceived).Msg(logging.ReceivedPreunits)

	log.Debug().Msg(logging.GetRequests)
	theirRequests, err := getRequests(nProc, posetInfo, conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.getRequests").Msg(err.Error())
		return
	}

	units, err := unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
	if err != nil {
		log.Error().Str("where", "proto.Out.unitsToSend").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.SendUnits)
	err = sendUnits(units, conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.sendUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(units)).Msg(logging.SentUnits)

	freshUnits, err := unitsToSend(poset, posetMaxSnapshot(poset), posetInfo, nil)
	if err != nil {
		log.Error().Str("where", "proto.Out.unitsToSend fresh").Msg(err.Error())
		return
	}
	theirPreunitsHashSet := newStaticHashSet(hashesFromAcquiredUnits(theirPreunitsReceived))
	freshUnitsUnknown := theirPreunitsHashSet.filterOutKnownUnits(freshUnits)

	log.Debug().Msg(logging.SendFreshUnits)
	err = sendUnits(freshUnitsUnknown, conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.sendUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(freshUnitsUnknown)).Msg(logging.SentFreshUnits)
	req := requestsToSend(poset, theirPosetInfo, theirPreunitsHashSet)
	log.Debug().Msg(logging.SendRequests)
	err = sendRequests(req, theirPosetInfo, conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.sendRequests").Msg(err.Error())
		return
	}

	if nonempty(req) {
		log.Info().Msg(logging.AdditionalExchange)
		log.Debug().Msg(logging.GetPreunits)
		theirPreunitsReceived, nReceived, err = getPreunits(conn)
		if err != nil {
			log.Error().Str("where", "proto.Out.getPreunits(extra round)").Msg(err.Error())
			return
		}
		log.Debug().Int(logging.Size, nReceived).Msg(logging.ReceivedPreunits)
	}

	log.Debug().Msg(logging.AddUnits)
	err = addUnits(poset, theirPreunitsReceived, pid, attemptTiming, log)
	if err != nil {
		log.Error().Str("where", "proto.Out.addUnits").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Sent, len(units)).Int(logging.Recv, nReceived).Msg(logging.SyncCompleted)
}
