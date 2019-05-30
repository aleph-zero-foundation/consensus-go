package request

import (
	"sync"
	"time"

	"github.com/rs/zerolog"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// In implements the side of the protocol that handles incoming connections.
type In struct {
	Timeout time.Duration
	Log     zerolog.Logger
}

// Out implements the side of the protocol that handles outgoing connections.
type Out struct {
	Timeout time.Duration
	Log     zerolog.Logger
}

func sendPosetInfo(info posetInfo, conn network.Connection) error {
	for _, pi := range info {
		err := encodeProcessInfo(conn, &pi)
		if err != nil {
			return err
		}
	}
	return nil
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

func sendUnits(units [][]gomel.Unit, conn network.Connection) error {
	return encodeUnits(conn, units)
}

func getPreunits(conn network.Connection) ([][]gomel.Preunit, int, error) {
	return decodeUnits(conn)
}

func sendRequests(req requests, conn network.Connection) error {
	return encodeRequests(conn, &req)
}

func getRequests(nProc int, conn network.Connection) (requests, error) {
	result, err := decodeRequests(conn, nProc)
	return *result, err
}

func addAntichain(poset gomel.Poset, preunits []gomel.Preunit, log zerolog.Logger) error {
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
func addUnits(poset gomel.Poset, preunits [][]gomel.Preunit, log zerolog.Logger) error {
	for _, pus := range preunits {
		err := addAntichain(poset, pus, log)
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

// Run handles the incoming connection using info from the poset.
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
func (p *In) Run(poset gomel.Poset, conn network.Connection) {
	defer conn.Close()
	conn.TimeoutAfter(p.Timeout)
	nProc := poset.NProc()
	log := p.Log.With().Uint16(logging.PID, conn.Pid()).Uint32(logging.ISID, conn.Sid()).Logger()
	log.Info().Msg(logging.SyncStarted)

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

	units, nSent := unitsToSend(poset, maxSnapshot, theirPosetInfo, make(requests, len(theirPosetInfo)))
	log.Debug().Msg(logging.SendUnits)
	err = sendUnits(units, conn)
	if err != nil {
		log.Error().Str("where", "proto.In.sendUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nSent).Msg(logging.SentUnits)

	req := requestsToSend(poset, theirPosetInfo, make([][]gomel.Preunit, len(theirPosetInfo)))
	log.Debug().Msg(logging.SendRequests)
	err = sendRequests(req, conn)
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

	log.Debug().Msg(logging.GetRequests)
	theirRequests, err := getRequests(nProc, conn)
	if err != nil {
		log.Error().Str("where", "proto.In.getRequests").Msg(err.Error())
		return
	}

	if nonempty(theirRequests) {
		log.Info().Msg(logging.AdditionalExchange)
		units, nSent = unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
		log.Debug().Msg(logging.SendUnits)
		err = sendUnits(units, conn)
		if err != nil {
			log.Error().Str("where", "proto.In.sendUnits(extra round)").Msg(err.Error())
			return
		}
		log.Debug().Int(logging.Size, nSent).Msg(logging.SentUnits)
	}

	log.Debug().Msg(logging.AddUnits)
	err = addUnits(poset, theirPreunitsReceived, log)
	if err != nil {
		log.Error().Str("where", "proto.In.addUnits").Msg(err.Error())
		return
	}
	log.Info().Int(logging.UnitsSent, nSent).Int(logging.UnitsRecv, nReceived).Msg(logging.SyncCompleted)
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
func (p *Out) Run(poset gomel.Poset, conn network.Connection) {
	defer conn.Close()
	conn.TimeoutAfter(p.Timeout)
	nProc := poset.NProc()
	log := p.Log.With().Uint32(logging.OSID, conn.Sid()).Logger()
	log.Info().Msg(logging.SyncStarted)

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
	theirRequests, err := getRequests(nProc, conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.getRequests").Msg(err.Error())
		return
	}

	units, nSent := unitsToSend(poset, maxSnapshot, theirPosetInfo, theirRequests)
	log.Debug().Msg(logging.SendUnits)
	err = sendUnits(units, conn)
	if err != nil {
		log.Error().Str("where", "proto.Out.sendUnits").Msg(err.Error())
		return
	}
	log.Debug().Int(logging.Size, nSent).Msg(logging.SentUnits)

	req := requestsToSend(poset, theirPosetInfo, theirPreunitsReceived)
	log.Debug().Msg(logging.SendRequests)
	err = sendRequests(req, conn)
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
	err = addUnits(poset, theirPreunitsReceived, log)
	if err != nil {
		log.Error().Str("where", "proto.Out.addUnits").Msg(err.Error())
		return
	}
	log.Info().Int(logging.UnitsSent, nSent).Int(logging.UnitsRecv, nReceived).Msg(logging.SyncCompleted)
}
