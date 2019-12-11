package gossip

import (
	"math/rand"
	"sync/atomic"
)

// peerManager represents a mechanism for getting pids of committee members, with whom we want to initiate a gossip.
// inUse semantics:
// 0 - not syncing
// 1 - incoming sync
// 2 - outgoing sync initiated with request
// 3 - outgoing sync initiated by idle
// 4 - our own pid
// idle channel is used to simulate a set of idleCap tokens that are used to limit the number of syncs happening
// in absence of external requests.
type peerManager struct {
	nProc    uint16
	inUse    []int64
	idle     chan struct{}
	requests chan uint16
	quit     int64
}

// newPeerManager constructs a peer manager for the member (identified by myPid) of a committee of size nProc.
// idle indicates how many syncs should happen simultaneously in absence of any external requests.
func newPeerManager(nProc, myPid uint16, idleCap int) *peerManager {
	idle := make(chan struct{}, idleCap)
	for i := 0; i < idleCap; i++ {
		idle <- struct{}{}
	}
	inUse := make([]int64, nProc)
	inUse[myPid] = 4
	return &peerManager{
		nProc:    nProc,
		inUse:    inUse,
		idle:     idle,
		requests: make(chan uint16, 5*nProc),
	}
}

// nextPeer returns the pid of next committee member we should gossip with.
func (pm *peerManager) nextPeer() (uint16, bool) {
	var pid uint16
	for {
		select {
		case pid = <-pm.requests:
		default:
			select {
			case pid = <-pm.requests:
			case _, ok := <-pm.idle:
				if !ok {
					return 0, false
				}
				pid = uint16(rand.Intn(int(pm.nProc)))
				if atomic.CompareAndSwapInt64(&pm.inUse[pid], 0, 3) {
					return pid, true
				}
				pm.idle <- struct{}{}
			}
		}
		if atomic.CompareAndSwapInt64(&pm.inUse[pid], 0, 2) {
			return pid, true
		}
	}
}

// begin notifies peerManager that we are starting incoming sync with the given committee member.
// Returns false if another sync with that PID is already happening
func (pm *peerManager) begin(pid uint16) bool {
	return atomic.CompareAndSwapInt64(&pm.inUse[pid], 0, 1)
}

// done notifies peerManager that a single gossip with the given committee member has finished.
func (pm *peerManager) done(pid uint16) {
	if atomic.CompareAndSwapInt64(&pm.inUse[pid], 3, 0) {
		if atomic.LoadInt64(&pm.quit) == 0 {
			pm.idle <- struct{}{}
		}
		return
	}
	atomic.StoreInt64(&pm.inUse[pid], 0)
}

// request the next sync to happen with the given committee member.
func (pm *peerManager) request(pid uint16) {
	if atomic.LoadInt64(&pm.inUse[pid]) > 0 {
		// sync with pid is already happening. Don't do it again.
		return
	}
	select {
	case pm.requests <- pid:
	default:
	}
}

func (pm *peerManager) stop() {
	atomic.StoreInt64(&pm.quit, 1)
	close(pm.idle)
}
