package udp

import (
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type connServer struct {
	localAddr  *net.UDPAddr
	listenChan chan<- network.Connection
	exitChan   chan struct{}
	wg         sync.WaitGroup
	log        zerolog.Logger
}

// NewConnServer creates and initializes a new connServer at the given localAddr pushing any connections into connSink.
func NewConnServer(localAddr string, connSink chan<- network.Connection, log zerolog.Logger) (network.ConnectionServer, error) {
	localUDP, err := net.ResolveUDPAddr("udp", localAddr)
	if err != nil {
		return nil, err
	}

	return &connServer{
		localAddr:  localUDP,
		listenChan: connSink,
		exitChan:   make(chan struct{}),
		log:        log,
	}, nil
}

func (cs *connServer) Start() error {
	ln, err := net.ListenUDP("udp", cs.localAddr)
	if err != nil {
		return err
	}
	cs.wg.Add(1)
	go func() {
		buffer := make([]byte, (1 << 16))
		for {
			select {
			case <-cs.exitChan:
				close(cs.listenChan)
				cs.wg.Done()
				return
			default:
				ln.SetDeadline(time.Now().Add(time.Second * 2))
				n, _, err := ln.ReadFromUDP(buffer)
				if err != nil {
					cs.log.Error().Str("where", "UDP connServer.Start").Msg(err.Error())
					continue
				}

				cs.listenChan <- NewConnIn(buffer[:n], cs.log)
				cs.log.Info().Msg(logging.ConnectionReceived)
			}
		}
	}()

	return nil
}

func (cs *connServer) Stop() {
	close(cs.exitChan)
	cs.wg.Wait()
}
