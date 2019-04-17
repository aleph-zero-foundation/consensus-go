package stdlibtcp

import (
	"net"
	"sync"
	"sync/atomic"
)

type channelServer struct {
	localAddr string
	channels  []*channel
	exitChan  chan struct{}
}

// NewChannelServer creates a new instance of object implementing network.ChannelServer interface
func NewChannelServer(localAddr, addresses []string) network.ChannelServer {
	chanServ := channelServer{
		localAddr: localAddr,
		channels:  make([]*channel, len(addresses)),
		exitChan:  make(chan struct{}),
	}
	for i, remoteAddr := range addresses {
		if remoteAddr == localAddr {
			continue
		}
		chanServ.channels[i] = newChannel(localAddr, remoteAddr)
	}
	return &chanServ
}

func (cs *channelServer) Start() {
	ln, err := net.ListenTCP("tcp", cs.localAddr)
	if err != nil {
		// handle error
	}
	for {
		select {
		case <-cs.exitChan:
			for _, channel := range cs.channels {
				defer channel.Close()
			}
			return
		default:
			conn, err := ln.AcceptTCP()
			if err != nil {
				// handle error
			}
			go cs.activateChannel(conn)
		}
	}
}

func (cs *channelServer) Stop() {
	close(cs.exitChan)
}

func (cs *channelServer) Channels() []network.Channel {
	return cs.channels
}

func (cs *channelServer) activateChannel(conn net.TCPConn) {
	remoteAddr := conn.RemoteAddr().String()
	for _, channel := range cs.channels {
		if channel.remoteAddr != remoteAddr {
			continue
		}
		channel.activate.Do(func() {
			channel.connection = conn
		}())
	}
}
