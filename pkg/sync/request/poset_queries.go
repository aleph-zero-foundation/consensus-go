package request

import (
	"sort"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

type unitInfo struct {
	hash   gomel.Hash
	height int
}

func toInfo(unit gomel.Unit) *unitInfo {
	return &unitInfo{*unit.Hash(), unit.Height()}
}

func toPosetInfo(maxSnapshot [][]gomel.Unit) [][]*unitInfo {
	result := [][]*unitInfo{}
	for _, units := range maxSnapshot {
		infoHere := []*unitInfo{}
		for _, u := range units {
			infoHere = append(infoHere, toInfo(u))
		}
		result = append(result, infoHere)
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
				newMaxes = append(newMaxes, p)
			}
			maxes[creator] = newMaxes
			maxes = fixMaximal(p, maxes)
		}
	}
	return maxes
}

func consistentMaximal(maxes [][]gomel.Unit) [][]gomel.Unit {
	for _, units := range maxes {
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
	return consistentMaximal(maxUnits)
}

func minimalHeight(info []*unitInfo) int {
	result := -1
	for _, i := range info {
		if i.height < result || result == -1 {
			result = i.height
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

func hashesSetFromInfo(info []*unitInfo) map[gomel.Hash]bool {
	result := map[gomel.Hash]bool{}
	for _, i := range info {
		result[i.hash] = true
	}
	return result
}

func hashesSliceFromInfo(info []*unitInfo) []gomel.Hash {
	result := []gomel.Hash{}
	for _, i := range info {
		result = append(result, i.hash)
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

func unitsToSendByProcess(tops []*unitInfo, maxes []gomel.Unit) []gomel.Unit {
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

func knownUnits(poset gomel.Poset, info []*unitInfo) map[gomel.Unit]bool {
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

func requestedToSend(poset gomel.Poset, info []*unitInfo, requests []gomel.Hash) []gomel.Unit {
	result := []gomel.Unit{}
	if len(requests) == 0 {
		return result
	}
	units := poset.Get(requests)
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
	if layer[u] != 0 {
		return layer[u]
	}
	maxParentLayer := 0
	for _, v := range u.Parents() {
		if computeLayer(v, layer) > maxParentLayer {
			maxParentLayer = computeLayer(v, layer)
		}
	}
	return maxParentLayer + 1
}

func toLayers(units []gomel.Unit) [][]gomel.Unit {
	layer := map[gomel.Unit]int{}
	maxLayer := 0
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

func unitsToSend(poset gomel.Poset, maxUnits [][]gomel.Unit, info [][]*unitInfo, requests [][]gomel.Hash) [][]gomel.Unit {
	toSend := []gomel.Unit{}
	for i := range info {
		toSendPid := unitsToSendByProcess(info[i], maxUnits[i])
		unfulfilledRequests := fiterOutKnown(requests[i], hashesFromUnits(toSendPid))
		toSendPid = append(toSendPid, requestedToSend(poset, info[i], unfulfilledRequests)...)
		toSend = append(toSend, toSendPid...)
	}
	return toLayers(toSend)
}

func unknownHashes(poset gomel.Poset, info []*unitInfo) []gomel.Hash {
	result := []gomel.Hash{}
	units := poset.Get(hashesSliceFromInfo(info))
	for i, u := range units {
		if u == nil {
			result = append(result, info[i].hash)
		}
	}
	return result
}

func requestsToSend(poset gomel.Poset, info [][]*unitInfo) [][]gomel.Hash {
	result := [][]gomel.Hash{}
	for i := range info {
		result = append(result, unknownHashes(poset, info[i]))
	}
	return result
}
