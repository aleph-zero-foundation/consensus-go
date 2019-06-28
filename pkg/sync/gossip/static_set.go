package gossip

import (
	"sort"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
)

func hashesFromInfo(info processInfo) []*gomel.Hash {
	result := make([]*gomel.Hash, len(info))
	for i, in := range info {
		result[i] = in.hash
	}
	return result
}

func hashesFromUnits(units []gomel.Unit) []*gomel.Hash {
	result := make([]*gomel.Hash, len(units))
	for i, u := range units {
		result[i] = u.Hash()
	}
	return result
}

type staticHashSet struct {
	hashes []*gomel.Hash
}

func newStaticHashSet(hashes []*gomel.Hash) staticHashSet {
	sort.Slice(hashes, func(i, j int) bool {
		return hashes[i].LessThan(hashes[j])
	})
	return staticHashSet{
		hashes: hashes,
	}
}

func (shs staticHashSet) contains(h *gomel.Hash) bool {
	idx := sort.Search(len(shs.hashes), func(i int) bool {
		return h.LessThan(shs.hashes[i])
	})
	if idx == 0 {
		return false
	}
	idx--
	return *shs.hashes[idx] == *h
}

func (shs staticHashSet) fiterOutKnown(hashes []*gomel.Hash) []*gomel.Hash {
	result := []*gomel.Hash{}
	for _, h := range hashes {
		if !shs.contains(h) {
			result = append(result, h)
		}
	}
	return result
}
