package request

import (
	"sort"
	"sync"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type unitInfo struct {
	hash   gomel.Hash
	height uint32
}

type processInfo []*unitInfo

type posetInfo []processInfo

type processRequests []gomel.Hash

type requests []processRequests

func toInfo(unit gomel.Unit) *unitInfo {
	return &unitInfo{*unit.Hash(), uint32(unit.Height())}
}

func toPosetInfo(maxSnapshot [][]gomel.Unit) posetInfo {
	result := make(posetInfo, len(maxSnapshot))
	for i, units := range maxSnapshot {
		infoHere := make(processInfo, len(units))
		for j, u := range units {
			infoHere[j] = toInfo(u)
		}
		result[i] = infoHere
	}
	return result
}

func belowAny(u gomel.Unit, units []gomel.Unit) bool {
	for _, v := range units {
		if u.Below(v) {
			return true
		}
	}
	return false
}

func fixMaximal(u gomel.Unit, maxes [][]gomel.Unit) [][]gomel.Unit {
	for _, p := range u.Parents() {
		creator := p.Creator()
		if !belowAny(p, maxes[creator]) {
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

func posetMaxSnapshot(poset gomel.Poset) [][]gomel.Unit {
	maxUnits := [][]gomel.Unit{}
	poset.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		maxUnits = append(maxUnits, units)
		return true
	})
	// The maximal units constructed through iterate might be inconsistent, i.e. contain units with parents that are not below any of their creators "maximal units".
	// Because of that, we fix this in a potentially expensive procedure. We don't expect this to be particularly bad in most cases, but we should make sure in some experiments.
	// TODO: Make sure in some experiments.
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

func hashesSetFromInfo(info processInfo) map[gomel.Hash]bool {
	result := map[gomel.Hash]bool{}
	for _, i := range info {
		result[i.hash] = true
	}
	return result
}

func hashesSliceFromInfo(info processInfo) []gomel.Hash {
	result := make([]gomel.Hash, len(info))
	for i, in := range info {
		result[i] = in.hash
	}
	return result
}

func hashesFromUnits(units []gomel.Unit) map[gomel.Hash]bool {
	result := map[gomel.Hash]bool{}
	for _, u := range units {
		result[*u.Hash()] = true
	}
	return result
}

// unitsToSendByProcess returns the units that are predecessors of maxes and successors of tops.
// A special case occurs when there are no tops, but maxes exist --
// then simply all predecessors of maxes are returned.
// This is to avoid the fourth round of the protocol in initial exchanges in cases with no forks.
func unitsToSendByProcess(tops processInfo, maxes []gomel.Unit) []gomel.Unit {
	result := []gomel.Unit{}
	sort.Slice(maxes, func(i, j int) bool {
		return maxes[i].Height() < maxes[j].Height()
	})
	minimalRemoteHeight := minimalHeight(tops)
	remoteHashes := hashesSetFromInfo(tops)
	for _, u := range maxes {
		possiblySend := []gomel.Unit{}
		for u.Height() >= minimalRemoteHeight {
			if remoteHashes[*u.Hash()] {
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

func fiterOutKnown(hashes []gomel.Hash, known map[gomel.Hash]bool) []gomel.Hash {
	result := []gomel.Hash{}
	for _, h := range hashes {
		if !known[h] {
			result = append(result, h)
		}
	}
	return result
}

func knownUnits(poset gomel.Poset, info processInfo) map[gomel.Unit]bool {
	allUnits := poset.Get(hashesSliceFromInfo(info))
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

func requestedToSend(poset gomel.Poset, info processInfo, req processRequests) []gomel.Unit {
	result := []gomel.Unit{}
	if len(req) == 0 {
		return result
	}
	units := poset.Get(req)
	operationHeight := maximalHeight(units)
	knownRemotes := knownUnits(poset, info)
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
	return result
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
func toLayers(units []gomel.Unit) ([][]gomel.Unit, int) {
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
	return result, len(units)
}

func unitsToSend(poset gomel.Poset, maxUnits [][]gomel.Unit, info posetInfo, req requests) ([][]gomel.Unit, int) {
	toSendPid := make([][]gomel.Unit, len(info))
	var wg sync.WaitGroup
	wg.Add(len(info))
	for i := range info {
		go func(id int) {
			toSendPid[id] = unitsToSendByProcess(info[id], maxUnits[id])
			unfulfilledRequests := fiterOutKnown(req[id], hashesFromUnits(toSendPid[id]))
			toSendPid[id] = append(toSendPid[id], requestedToSend(poset, info[id], unfulfilledRequests)...)
			wg.Done()
		}(i)
	}
	wg.Wait()
	toSend := []gomel.Unit{}
	for _, tsp := range toSendPid {
		toSend = append(toSend, tsp...)
	}
	return toLayers(toSend)
}

func unknownHashes(poset gomel.Poset, info processInfo, alsoKnown [][]gomel.Preunit) processRequests {
	result := processRequests{}
	units := poset.Get(hashesSliceFromInfo(info))
	for i, u := range units {
		if u == nil {
			actuallyKnown := false
			// TODO: This might be slow and might happen often. Think about optimizing.
			// Note that this happens in separate goroutines, so might not be a bottleneck.
			for _, units := range alsoKnown {
				for _, v := range units {
					if *v.Hash() == info[i].hash {
						actuallyKnown = true
						break
					}
				}
			}
			if !actuallyKnown {
				result = append(result, info[i].hash)
			}
		}
	}
	return result
}

func requestsToSend(poset gomel.Poset, info posetInfo, aquiredUnits [][]gomel.Preunit) requests {
	result := make(requests, len(info))
	var wg sync.WaitGroup
	wg.Add(len(info))
	for i := range info {
		go func(id int) {
			result[id] = unknownHashes(poset, info[id], aquiredUnits)
			wg.Done()
		}(i)
	}
	wg.Wait()
	return result
}
