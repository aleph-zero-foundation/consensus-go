package tcp

import (
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type connServer struct {
	localAddr  *net.TCPAddr
	listenChan chan<- network.Connection
	exitChan   chan struct{}
	wg         sync.WaitGroup
	log        zerolog.Logger
}

// NewConnServer creates and initializes a new connServer at the given localAddr pushing any connections into connSink.
func NewConnServer(localAddr string, connSink chan<- network.Connection, log zerolog.Logger) (network.ConnectionServer, error) {
	localTCP, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}

	return &connServer{
		localAddr:  localTCP,
		listenChan: connSink,
		exitChan:   make(chan struct{}),
		log:        log,
	}, nil
}

func (cs *connServer) Start() error {
	ln, err := net.ListenTCP("tcp", cs.localAddr)
	if err != nil {
		return err
	}
	cs.wg.Add(1)
	go func() {
		for {
			select {
			case <-cs.exitChan:
				close(cs.listenChan)
				cs.wg.Done()
				return
			default:
				ln.SetDeadline(time.Now().Add(time.Second * 2))
				link, err := ln.AcceptTCP()
				if err != nil {
					cs.log.Error().Str("where", "connServer.Listen").Msg(err.Error())
					continue
				}
				conn := NewConn(link, 0, 0, cs.log)
				select {
				case cs.listenChan <- conn:
					cs.log.Info().Msg(logging.ConnectionReceived)
				default:
					link.Close()
					cs.log.Info().Msg(logging.TooManyIncoming)
				}
			}
		}
	}()

	return nil
}

func (cs *connServer) Stop() {
	close(cs.exitChan)
	cs.wg.Wait()
}
