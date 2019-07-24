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

// NewDefaultPeerSource creates a peer source that randomly chooses a pid to sync with.
func NewDefaultPeerSource(nProc, myPid uint16) PeerSource {
	// compute a uniform distribution on [n]\{id}
	p := 1.0 / float64(nProc-1)
	dist := make([]float64, nProc)
	for i := range dist {
		dist[i] = p
	}
	dist[myPid] = 0
	for i := uint16(1); i < nProc; i++ {
		dist[i] += dist[i-1]
	}
	return &defaultPeerSource{
		nProc: nProc,
		myPid: myPid,
		dist:  dist,
	}
}

type defaultPeerSource struct {
	nProc uint16
	myPid uint16
	dist  []float64
}

func (ps *defaultPeerSource) NextPeer() uint16 {
	return uint16(sort.SearchFloat64s(ps.dist, rand.Float64()))
}

// NewChanPeerSource creates a peer source that gets the pids from the provided channel.
func NewChanPeerSource(source <-chan uint16) PeerSource {
	return &chanPeerSource{source}
}

type chanPeerSource struct {
	source <-chan uint16
}

func (ps *chanPeerSource) NextPeer() uint16 {
	return <-ps.source
}

// NewMixedPeerSource creates a peer source that mixes functionalities of default and chan peer sources.
// It tries to read the next pid from the provided channel. If the channel is empty,
// it behaves like the default peer source, randomly choosing a pid other than myPid
//
func NewMixedPeerSource(nProc, myPid uint16, source <-chan uint16) PeerSource {
	dps := NewDefaultPeerSource(nProc, myPid).(*defaultPeerSource)
	return &mixedPeerSource{dps, source}
}

type mixedPeerSource struct {
	dps    *defaultPeerSource
	source <-chan uint16
}

func (ps *mixedPeerSource) NextPeer() uint16 {
	var pid uint16
	select {
	case pid = <-ps.source:
	default:
		pid = ps.dps.NextPeer()
	}
	return pid

}
