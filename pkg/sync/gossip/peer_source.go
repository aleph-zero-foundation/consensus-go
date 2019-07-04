package gossip

import (
	"math/rand"
	"sort"
)

type peerSource struct {
	nProc uint16
	id    uint16
	dist  []float64
}

func newPeerSource(nProc, id uint16) *peerSource {
	// compute a uniform distribution on [n]\{id}
	p := 1.0 / float64(nProc-1)
	dist := make([]float64, nProc)
	for i := range dist {
		dist[i] = p
	}
	dist[id] = 0
	for i := uint16(1); i < nProc; i++ {
		dist[i] += dist[i-1]
	}
	return &peerSource{
		nProc: nProc,
		id:    id,
		dist:  dist,
	}
}

// nextPeer returns a pid of next peer to call to
// Note: it is thread-safe
func (ps *peerSource) nextPeer() uint16 {
	return uint16(sort.SearchFloat64s(ps.dist, rand.Float64()))
}
