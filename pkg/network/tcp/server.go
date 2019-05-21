package tcp

import (
	"net"

	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type connServer struct {
	pid         int
	localAddr   *net.TCPAddr
	remoteAddrs []*net.TCPAddr
	listenChan  chan network.Connection
	dialChan    chan network.Connection
	dialSource  <-chan int
	inUse       []*mutex
	syncIds     []uint32
	exitChan    chan struct{}
}

// NewConnServer creates and initializes a new connServer with given localAddr, remoteAddrs, dialSource, and queue lengths for listens and syncs.
func NewConnServer(localAddr string, remoteAddrs []string, dialSource <-chan int, listQueueLen, syncQueueLen int) (network.ConnectionServer, error) {
	localTCP, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	remoteTCPs := make([]*net.TCPAddr, len(remoteAddrs))
	inUse := make([]*mutex, len(remoteAddrs))
	pid := -1
	for i, remoteAddr := range remoteAddrs {
		remoteTCP, err := net.ResolveTCPAddr("tcp", remoteAddr)
		if err != nil {
			return nil, err
		}
		remoteTCPs[i] = remoteTCP
		inUse[i] = newMutex()
		if remoteTCP == localTCP {
			pid = i
		}
	}

	return &connServer{
		pid:         pid,
		localAddr:   localTCP,
		remoteAddrs: remoteTCPs,
		listenChan:  make(chan network.Connection, listQueueLen),
		dialChan:    make(chan network.Connection, syncQueueLen),
		dialSource:  dialSource,
		inUse:       inUse,
		syncIds:     make([]uint32, len(remoteAddrs)),
		exitChan:    make(chan struct{}),
	}, nil
}

func (cs *connServer) ListenChannel() <-chan network.Connection {
	return cs.listenChan
}

func (cs *connServer) DialChannel() <-chan network.Connection {
	return cs.dialChan
}

func (cs *connServer) Listen() error {
	ln, err := net.ListenTCP("tcp", cs.localAddr)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-cs.exitChan:
				close(cs.listenChan)
				return
			default:
				link, err := ln.AcceptTCP()
				if err != nil {
					// TODO log the error
				}
				g, err := getGreeting(link)
				if err != nil {
					// TODO log the error
					continue
				}
				remotePid := int(g.pid)
				if remotePid < 0 || remotePid >= len(cs.inUse) {
					// TODO log that a stranger called us
					continue
				}
				// check if we are already syncing with remotePeer
				m := cs.inUse[remotePid]
				if m.tryAcquire() {
					cs.listenChan <- newConn(link, m)
				}
			}
		}
	}()

	return nil
}

func (cs *connServer) StartDialing() {
	go func() {
		for {
			select {
			case <-cs.exitChan:
				close(cs.dialChan)
				return
			case remotePid := <-cs.dialSource:
				// check if we are already syncing with remotePeer
				m := cs.inUse[remotePid]
				if !m.tryAcquire() {
					continue
				}

				link, err := net.DialTCP("tcp", nil, cs.remoteAddrs[remotePid])
				if err != nil {
					// TODO log the error
					m.release()
				}
				g := &greeting{
					pid: uint32(remotePid),
					sid: cs.syncIds[remotePid],
				}
				cs.syncIds[remotePid]++
				err = g.send(link)
				if err != nil {
					// TODO log the error
					m.release()
					continue
				}
				cs.dialChan <- newConn(link, m)
			}
		}
	}()

}

func (cs *connServer) Stop() {
	close(cs.exitChan)
}
