package sync

import (
	"math/rand"
	"sort"
)

// dialerChan generates remote peers' pids for outgoing synchronizations
type dialer struct {
	nProc uint16
	id    uint16
	dist  []float64
}

func newDialer(nProc, id uint16) *dialer {
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
	return &dialer{
		nProc: nProc,
		id:    id,
		dist:  dist,
	}
}

// nextPeer returns a pid of next peer to call to
// Note: it is thread-safe
func (d *dialer) nextPeer() uint16 {
	return uint16(sort.SearchFloat64s(d.dist, rand.Float64()))
}
