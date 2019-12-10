package gossip

import ()

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

type peerManager struct {
	nProc uint16
	myPid uint16
	idle  int
}

// NewPeerManager constructs a peer manager for the member (identified by myPid) of a committee of size nProc.
// idle indicates how many syncs should happen simultaneously in absence of any external requests.
func NewPeerManager(nProc, myPid uint16, idle int) PeerManager {
	return &peerManager{
		nProc: nProc,
		myPid: myPid,
		idle:  idle,
	}
}

func (pm *peerManager) NextPeer() uint16 {
	return 0
}

func (pm *peerManager) Begin(pid uint16) bool {
	return true
}

func (pm *peerManager) Done(pid uint16) {}

func (pm *peerManager) Request(pid uint16) {}
