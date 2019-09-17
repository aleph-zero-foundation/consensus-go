package gossip

import (
	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync/add"
)

func sendDagInfo(info dagInfo, conn network.Connection) error {
	for _, pi := range info {
		err := encodeProcessInfo(conn, pi)
		if err != nil {
			return err
		}
	}
	return nil
}

func getDagInfo(nProc uint16, conn network.Connection) (dagInfo, error) {
	info := make(dagInfo, nProc)
	for i := range info {
		pi, err := decodeProcessInfo(conn)
		if err != nil {
			return nil, err
		}
		info[i] = pi
	}
	return info, nil
}

// addUnits adds the provided units to the dag, assuming they are divided into antichains as described in toLayers
func (p *server) addUnits(preunits [][]gomel.Preunit, log zerolog.Logger) error {
	for _, pus := range preunits {
		err := add.Antichain(p.dag, p.randomSource, pus, p.fallback, log)
		if err != nil {
			return err
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

// inExchange handles the incoming connection using info from the dag.
// This version uses 3-exchange "pullpush" protocol: receive heights, send heights, units and requests, receive units and requests.
// If we receive some requests there is a 4th exchange where we once again send units. This should only happen due to forks.
//
// The precise flow of this protocol follows:
/*		1. Receive a consistent snapshot of the other parties maximal units as a list of (hash, height) pairs.
		2. Compute a similar info for our dag.
		3. Send this info.
		4. Compute and send units that are predecessors of the received info and successors of ours.
		5. Compute and send requests, containing hashes in the other party's info not recognized by us.
		6. Receive units complying with the above restrictions and the ones we requested.
		7. Receive requests as above. If they were empty proceed to 9.
		8. Add units that are requested and their predecessors down to the first we know they have, and send all the units.
		9. Add the received units to the dag.
*/
func (p *server) inExchange(conn network.Connection) {
	log := conn.Log()
	log.Info().Msg(logging.SyncStarted)
	nProc := p.dag.NProc()

	log.Debug().Msg(logging.GetDagInfo)
	theirDagInfo, err := getDagInfo(nProc, conn)
	if err != nil {
		log.Error().Str("where", "gossip.in.getDagInfo").Msg(err.Error())
		return
	}

	maxSnapshot := dagMaxSnapshot(p.dag)
	dagInfo := toDagInfo(maxSnapshot)
	log.Debug().Msg(logging.SendDagInfo)
	if err := sendDagInfo(dagInfo, conn); err != nil {
		log.Error().Str("where", "gossip.in.sendDagInfo").Msg(err.Error())
		return
	}

	units, err := unitsToSend(p.dag, maxSnapshot, theirDagInfo, nil)
	if err != nil {
		log.Error().Str("where", "gossip.in.unitsToSend").Msg(err.Error())
		return
	}
	log.Debug().Msg(logging.SendUnits)
	err = encodeUnits(conn, toLayers(units))
	if err != nil {
		log.Error().Str("where", "gossip.in.encodeUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(units)).Msg(logging.SentUnits)

	req := requestsToSend(p.dag, theirDagInfo, newStaticHashSet(nil))
	log.Debug().Msg(logging.SendRequests)
	err = encodeRequests(conn, req, theirDagInfo)
	if err != nil {
		log.Error().Str("where", "gossip.in.encodeRequests").Msg(err.Error())
		return
	}

	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "gossip.in.Flush").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.GetPreunits)
	theirPreunitsReceived, nReceived, err := decodeUnits(conn)
	if err != nil {
		log.Error().Str("where", "gossip.in.decodeUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nReceived).Msg(logging.ReceivedPreunits)

	log.Debug().Msg(logging.GetPreunits)
	theirFreshPreunitsReceived, nFreshReceived, err := decodeUnits(conn)
	if err != nil {
		log.Error().Str("where", "gossip.in.decodeUnits fresh").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nFreshReceived).Msg(logging.ReceivedPreunits)

	log.Debug().Msg(logging.GetRequests)
	theirRequests, err := decodeRequests(conn, dagInfo)
	if err != nil {
		log.Error().Str("where", "gossip.in.decodeRequests").Msg(err.Error())
		return
	}

	if nonempty(theirRequests) {
		log.Info().Msg(logging.AdditionalExchange)
		units, err = unitsToSend(p.dag, maxSnapshot, theirDagInfo, theirRequests)
		if err != nil {
			log.Error().Str("where", "gossip.in.unitsToSend(extra round)").Msg(err.Error())
			return
		}
		log.Debug().Msg(logging.SendUnits)
		err = encodeUnits(conn, toLayers(units))
		if err != nil {
			log.Error().Str("where", "gossip.in.encodeUnits(extra round)").Msg(err.Error())
			return
		}
		err = conn.Flush()
		if err != nil {
			log.Error().Str("where", "gossip.in.Flush(extra round)").Msg(err.Error())
			return
		}

		log.Debug().Int(logging.Size, len(units)).Msg(logging.SentUnits)
	}

	log.Debug().Msg(logging.AddUnits)
	err = p.addUnits(theirPreunitsReceived, log)
	if err != nil {
		log.Error().Str("where", "gossip.in.addUnits").Msg(err.Error())
		return
	}
	err = p.addUnits(theirFreshPreunitsReceived, log)
	if err != nil {
		log.Error().Str("where", "gossip.in.addUnits fresh").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Sent, len(units)).Int(logging.Recv, nReceived).Int(logging.FreshRecv, nFreshReceived).Msg(logging.SyncCompleted)
}

// Run handles the outgoing connection using info from the dag.
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
		10. Add the received units to the dag.
*/
func (p *server) outExchange(conn network.Connection) {
	log := conn.Log()
	log.Info().Msg(logging.SyncStarted)
	nProc := p.dag.NProc()

	maxSnapshot := dagMaxSnapshot(p.dag)
	dagInfo := toDagInfo(maxSnapshot)
	log.Debug().Msg(logging.SendDagInfo)
	if err := sendDagInfo(dagInfo, conn); err != nil {
		log.Error().Str("where", "gossip.out.sendDagInfo").Msg(err.Error())
		return
	}

	err := conn.Flush()
	if err != nil {
		log.Error().Str("where", "gossip.out.Flush(first)").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.GetDagInfo)
	theirDagInfo, err := getDagInfo(nProc, conn)
	if err != nil {
		log.Error().Str("where", "gossip.out.getDagInfo").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.GetPreunits)
	theirPreunitsReceived, nReceived, err := decodeUnits(conn)
	if err != nil {
		log.Error().Str("where", "gossip.out.decodeUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nReceived).Msg(logging.ReceivedPreunits)

	log.Debug().Msg(logging.GetRequests)
	theirRequests, err := decodeRequests(conn, dagInfo)
	if err != nil {
		log.Error().Str("where", "gossip.out.decodeRequests").Msg(err.Error())
		return
	}

	units, err := unitsToSend(p.dag, maxSnapshot, theirDagInfo, theirRequests)
	if err != nil {
		log.Error().Str("where", "gossip.out.unitsToSend").Msg(err.Error())
		return
	}

	log.Debug().Msg(logging.SendUnits)
	err = encodeUnits(conn, toLayers(units))
	if err != nil {
		log.Error().Str("where", "gossip.out.encodeUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(units)).Msg(logging.SentUnits)

	freshUnits, err := unitsToSend(p.dag, dagMaxSnapshot(p.dag), dagInfo, nil)
	if err != nil {
		log.Error().Str("where", "gossip.out.unitsToSend fresh").Msg(err.Error())
		return
	}
	theirPreunitsHashSet := newStaticHashSet(hashesFromAcquiredUnits(theirPreunitsReceived))
	freshUnitsUnknown := theirPreunitsHashSet.filterOutKnownUnits(freshUnits)

	log.Debug().Msg(logging.SendFreshUnits)
	err = encodeUnits(conn, toLayers(freshUnitsUnknown))
	if err != nil {
		log.Error().Str("where", "gossip.out.encodeUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, len(freshUnitsUnknown)).Msg(logging.SentFreshUnits)
	req := requestsToSend(p.dag, theirDagInfo, theirPreunitsHashSet)
	log.Debug().Msg(logging.SendRequests)
	err = encodeRequests(conn, req, theirDagInfo)
	if err != nil {
		log.Error().Str("where", "gossip.out.encodeRequests").Msg(err.Error())
		return
	}

	err = conn.Flush()
	if err != nil {
		log.Error().Str("where", "gossip.out.Flush(second)").Msg(err.Error())
		return
	}

	if nonempty(req) {
		log.Info().Msg(logging.AdditionalExchange)
		log.Debug().Msg(logging.GetPreunits)
		theirPreunitsReceived, nReceived, err = decodeUnits(conn)
		if err != nil {
			log.Error().Str("where", "gossip.out.decodeUnits(extra round)").Msg(err.Error())
			return
		}
		log.Debug().Int(logging.Size, nReceived).Msg(logging.ReceivedPreunits)
	}

	log.Debug().Msg(logging.AddUnits)
	err = p.addUnits(theirPreunitsReceived, log)
	if err != nil {
		log.Error().Str("where", "gossip.out.addUnits").Msg(err.Error())
		return
	}
	log.Info().Int(logging.Sent, len(units)).Int(logging.FreshSent, len(freshUnitsUnknown)).Int(logging.Recv, nReceived).Msg(logging.SyncCompleted)
}
