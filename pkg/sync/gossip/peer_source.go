package gossip

import (
	"math/rand"
	"sort"
)

// PeerSource represents a mechanism for getting pids of committee members, with whom we want to initiate a gossip.
type PeerSource interface {
	// NextPeer returns the pid of such a committee member.
	// It might block in some implementations, but should always be thread safe.
	NextPeer() uint16
}

type defaultPeerSource struct {
	nProc uint16
	id    uint16
	dist  []float64
}

// NewDefaultPeerSource creates a peer source that randomly chooses a pid to sync with.
func NewDefaultPeerSource(nProc, id uint16) PeerSource {
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
	return &defaultPeerSource{
		nProc: nProc,
		id:    id,
		dist:  dist,
	}
}

func (ps *defaultPeerSource) NextPeer() uint16 {
	return uint16(sort.SearchFloat64s(ps.dist, rand.Float64()))
}

type chanPeerSource struct {
	source <-chan uint16
}

// NewChanPeerSource creates a peer source that gets the pids from the provided channel.
func NewChanPeerSource(source <-chan uint16) PeerSource {
	return &chanPeerSource{source}
}

func (ps *chanPeerSource) NextPeer() uint16 {
	return <-ps.source
}
