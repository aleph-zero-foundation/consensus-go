package gossip

import (
	"sync"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type unitInfo struct {
	hash   *gomel.Hash
	height uint32
}

type processInfo []unitInfo

type dagInfo []processInfo

type processRequests []*gomel.Hash

type requests []processRequests

func toInfo(unit gomel.Unit) unitInfo {
	return unitInfo{unit.Hash(), uint32(unit.Height())}
}

func toDagInfo(maxSnapshot [][]gomel.Unit) dagInfo {
	result := make(dagInfo, len(maxSnapshot))
	for i, units := range maxSnapshot {
		infoHere := make(processInfo, len(units))
		for j, u := range units {
			infoHere[j] = toInfo(u)
		}
		result[i] = infoHere
	}
	return result
}

func fixMaximal(u gomel.Unit, maxes [][]gomel.Unit) [][]gomel.Unit {
	for _, p := range u.Parents() {
		creator := p.Creator()
		if !gomel.BelowAny(p, maxes[creator]) {
			newMaxes := []gomel.Unit{}
			for _, m := range maxes[creator] {
				if !m.Below(p) {
					newMaxes = append(newMaxes, m)
				}
			}
			newMaxes = append(newMaxes, p)
			maxes[creator] = newMaxes
			maxes = fixMaximal(p, maxes)
		}
	}
	return maxes
}

func consistentMaximal(maxes [][]gomel.Unit) [][]gomel.Unit {
	for i := range maxes {
		units := maxes[i]
		for _, u := range units {
			maxes = fixMaximal(u, maxes)
		}
	}
	return maxes
}

func dagMaxSnapshot(dag gomel.Dag) [][]gomel.Unit {
	maxUnits := [][]gomel.Unit{}
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		unitsCopy := make([]gomel.Unit, len(units))
		copy(unitsCopy, units)
		maxUnits = append(maxUnits, unitsCopy)
		return true
	})
	// The maximal units constructed through iterate might be inconsistent, i.e. contain units with parents that are not below any of their creators "maximal units".
	return consistentMaximal(maxUnits)
}

func minimalHeight(info processInfo) int {
	result := -1
	for _, i := range info {
		if int(i.height) < result || result == -1 {
			result = int(i.height)
		}
	}
	return result
}

func maximalHeight(units []gomel.Unit) int {
	result := -1
	for _, u := range units {
		if u.Height() > result {
			result = u.Height()
		}
	}
	return result
}

// unitsToSendByProcess returns the units that are predecessors of maxes and successors of tops.
// A special case occurs when there are no tops, but maxes exist --
// then simply all predecessors of maxes are returned.
// This is to avoid the fourth round of the protocol in initial exchanges in cases with no forks.
func unitsToSendByProcess(tops processInfo, maxes []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	minimalRemoteHeight := minimalHeight(tops)
	remoteHashes := newStaticHashSet(hashesFromInfo(tops))
	for _, u := range maxes {
		possiblySend := []gomel.Unit{}
		for u.Height() >= minimalRemoteHeight {
			if remoteHashes.contains(u.Hash()) {
				result = append(result, possiblySend...)
				break
			}
			possiblySend = append(possiblySend, u)
			v, err := gomel.Predecessor(u)
			if err != nil {
				result = append(result, possiblySend...)
				break
			}
			u = v
		}
	}
	return result
}

func knownUnits(dag gomel.Dag, info processInfo) map[gomel.Unit]bool {
	allUnits := dag.Get(hashesFromInfo(info))
	result := map[gomel.Unit]bool{}
	for _, u := range allUnits {
		if u != nil {
			result[u] = true
		}
	}
	return result
}

func dropToHeight(units map[gomel.Unit]bool, height int) map[gomel.Unit]bool {
	result := map[gomel.Unit]bool{}
	if height == -1 {
		return result
	}
	for u := range units {
		for u.Height() > height {
			u, _ = gomel.Predecessor(u)
		}
		result[u] = true
	}
	return result
}

func splitOffHeight(units []gomel.Unit, height int) ([]gomel.Unit, []gomel.Unit) {
	atHeight, rest := []gomel.Unit{}, []gomel.Unit{}
	for _, u := range units {
		if u.Height() == height {
			atHeight = append(atHeight, u)
		} else {
			rest = append(rest, u)
		}
	}
	return atHeight, rest
}

func requestedToSend(dag gomel.Dag, info processInfo, req processRequests) ([]gomel.Unit, error) {
	result := []gomel.Unit{}
	if len(req) == 0 {
		return result, nil
	}
	units := dag.Get(req)
	operationHeight := maximalHeight(units)
	knownRemotes := knownUnits(dag, info)
	knownRemotes = dropToHeight(knownRemotes, operationHeight)
	for len(units) > 0 {
		consideredUnits, units := splitOffHeight(units, operationHeight)
		for _, u := range consideredUnits {
			if !knownRemotes[u] {
				result = append(result, u)
				if v, err := gomel.Predecessor(u); err == nil {
					units = append(units, v)
				}
			}
		}
		operationHeight--
		knownRemotes = dropToHeight(knownRemotes, operationHeight)
	}
	return result, nil
}

func computeLayer(u gomel.Unit, layer map[gomel.Unit]int) int {
	if layer[u] == -1 {
		maxParentLayer := 0
		for _, v := range u.Parents() {
			if computeLayer(v, layer) > maxParentLayer {
				maxParentLayer = computeLayer(v, layer)
			}
		}
		layer[u] = maxParentLayer + 1
	}
	return layer[u]
}

// toLayers divides the provided units into antichains, so that each antichain is
// maximal, and depends only on units from outside or from previous antichains.
func toLayers(units []gomel.Unit) [][]gomel.Unit {
	layer := map[gomel.Unit]int{}
	maxLayer := 0
	for _, u := range units {
		layer[u] = -1
	}
	for _, u := range units {
		layer[u] = computeLayer(u, layer)
		if layer[u] > maxLayer {
			maxLayer = layer[u]
		}
	}
	result := make([][]gomel.Unit, maxLayer)
	for _, u := range units {
		result[layer[u]-1] = append(result[layer[u]-1], u)
	}
	return result
}

func unitsToSend(dag gomel.Dag, maxSnapshot [][]gomel.Unit, info dagInfo, req requests) ([]gomel.Unit, error) {
	nProc := dag.NProc()
	toSendPid := make([][]gomel.Unit, nProc)
	var err error
	var wg sync.WaitGroup
	wg.Add(nProc)
	for i := 0; i < nProc; i++ {
		go func(id int) {
			toSendPid[id] = unitsToSendByProcess(info[id], maxSnapshot[id])
			if req != nil {
				unfulfilledRequests := newStaticHashSet(hashesFromUnits(toSendPid[id])).fiterOutKnown(req[id])
				requested, e := requestedToSend(dag, info[id], unfulfilledRequests)
				if e != nil {
					err = e
				}
				toSendPid[id] = append(toSendPid[id], requested...)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	if err != nil {
		return nil, err
	}
	toSend := []gomel.Unit{}
	for _, tsp := range toSendPid {
		toSend = append(toSend, tsp...)
	}
	return toSend, nil
}

func unknownHashes(dag gomel.Dag, info processInfo, alsoKnown staticHashSet) processRequests {
	result := processRequests{}
	units := dag.Get(hashesFromInfo(info))
	for i, u := range units {
		if u == nil {
			if !alsoKnown.contains(info[i].hash) {
				result = append(result, info[i].hash)
			}
		}
	}
	return result
}

func requestsToSend(dag gomel.Dag, info dagInfo, alsoKnown staticHashSet) requests {
	result := make(requests, len(info))
	var wg sync.WaitGroup
	wg.Add(len(info))
	for i := range info {
		go func(id int) {
			result[id] = unknownHashes(dag, info[id], alsoKnown)
			wg.Done()
		}(i)
	}
	wg.Wait()
	return result
}