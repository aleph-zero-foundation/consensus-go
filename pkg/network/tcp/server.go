package tcp

import (
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"net"
)

const (
	N_LQUEUE = 10 // TODO move to config
	N_SQUEUE = 10 // TODO move to config
)

type connServer struct {
	localAddr   *net.TCPAddr
	remoteAddrs []*net.TCPAddr
	listenChan  chan network.Connection
	dialChan    chan network.Connection
	dialPolicy  func() int
	inUse       map[net.Addr]mutex
	exitChan    chan struct{}
}

func NewConnServer(localAddr string, remoteAddrs []string, dialPolicy func() int) (network.ConnectionServer, error) {
	localTCP, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	remoteTCPs := make([]*net.TCPAddr, len(remoteAddrs))
	inUse := make(map[net.Addr]mutex)
	for i, remoteAddr := range remoteAddrs {
		if remoteTCP, err := net.ResolveTCPAddr("tcp", remoteAddr); err != nil {
			return nil, err
		} else {
			remoteTCPs[i] = remoteTCP
			inUse[remoteTCP] = mutex{new(uint32)}
		}
	}

	return &connServer{
		localAddr:   localTCP,
		remoteAddrs: remoteTCPs,
		listenChan:  make(chan network.Connection, N_LQUEUE),
		dialChan:    make(chan network.Connection, N_SQUEUE),
		inUse:       inUse,
		exitChan:    make(chan struct{}),
	}, nil
}

func (cs *connServer) Listen() (chan network.Connection, error) {
	ln, err := net.ListenTCP("tcp", cs.localAddr)
	if err != nil {
		return nil, err
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
				// check if we are already syncing with remotePeer
				m := cs.inUse[link.RemoteAddr()]
				if m.tryAcquire() {
					cs.listenChan <- newConn(link, m)
				}
			}
		}
	}()

	return cs.listenChan, nil
}

func (cs *connServer) Dial() chan network.Connection {
	go func() {
		for {
			select {
			case <-cs.exitChan:
				close(cs.dialChan)
				return
			default:
				remotePid := cs.dialPolicy()
				// check if we are already syncing with remotePeer
				m := cs.inUse[cs.remoteAddrs[remotePid]]
				if !m.tryAcquire() {
					continue
				}

				link, err := net.DialTCP("tcp", nil, cs.remoteAddrs[remotePid])
				if err != nil {
					// TODO log the error
					m.release()
				}
				cs.dialChan <- newConn(link, m)
			}
		}
	}()

	return cs.dialChan
}

func (cs *connServer) Stop() {
	close(cs.exitChan)
}
