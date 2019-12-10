package gossip

import (
	"math/rand"
	"sync/atomic"
)

// PeerManager represents a mechanism for getting pids of committee members, with whom we want to initiate a gossip.
type PeerManager interface {
	// NextPeer returns the pid of next committee member we should gossip with.
	NextPeer() uint16
	// Begin notifies PeerManager that we are starting incoming sync with the given committee member.
	// Returns false if another sync with that PID is already happening
	Begin(uint16) bool
	// Done notifies PeerManager that a single gossip with the given committee member has finished.
	Done(uint16)
	// Request the next sync to happen with the given committee member.
	Request(uint16)
}

// inUse semantics:
// 0 - not syncing
// 1 - incoming sync
// 2 - outgoing sync initiated with Request
// 3 - outgoing sync initiated by idle
type peerManager struct {
	nProc int
	myPid uint16
	inUse []int64
	idle  chan struct{}
	queue chan uint16
}

// NewPeerManager constructs a peer manager for the member (identified by myPid) of a committee of size nProc.
// idle indicates how many syncs should happen simultaneously in absence of any external requests.
func NewPeerManager(nProc, myPid uint16, idleCap int) PeerManager {
	idle := make(chan struct{}, idleCap)
	for i := 0; i < idleCap; i++ {
		idle <- struct{}{}
	}
	return &peerManager{
		nProc: int(nProc),
		myPid: myPid,
		inUse: make([]int64, nProc),
		idle:  idle,
		queue: make(chan uint16, nProc),
	}
}

func (pm *peerManager) NextPeer() uint16 {
	var pid uint16
	for {
		select {
		case pid = <-pm.queue:
		default:
			select {
			case pid = <-pm.queue:
			case <-pm.idle:
				pid = uint16(rand.Intn(pm.nProc))
				if atomic.CompareAndSwapInt64(&pm.inUse[pid], 0, 3) {
					return pid
				}
				pm.idle <- struct{}{}
			}
		}
		if atomic.CompareAndSwapInt64(&pm.inUse[pid], 0, 2) {
			return pid
		}
	}
}

func (pm *peerManager) Begin(pid uint16) bool {
	return atomic.CompareAndSwapInt64(&pm.inUse[pid], 0, 1)
}

func (pm *peerManager) Done(pid uint16) {
	if atomic.CompareAndSwapInt64(&pm.inUse[pid], 3, 0) {
		pm.idle <- struct{}{}
		return
	}
	atomic.StoreInt64(&pm.inUse[pid], 0)
}

func (pm *peerManager) Request(pid uint16) {
	if atomic.LoadInt64(&pm.inUse[pid]) > 0 {
		// sync with pid is already happening. Don't do it again.
		return
	}
	select {
	case pm.queue <- pid:
	default:
	}
}
