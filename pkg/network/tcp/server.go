package tcp

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

type connServer struct {
	outboundAddr *net.TCPAddr
	localAddr    *net.TCPAddr
	remoteAddrs  []string
	listenChan   chan net.Conn
	dialChan     chan net.Conn
	dialSource   <-chan int
	inUse        []*gsync.Mutex
	exitChan     chan struct{}
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewConnServer creates and initializes a new connServer with given localAddr, remoteAddrs, dialSource, and queue lengths for listens and syncs.
func NewConnServer(localAddr string, remoteAddrs []string, dialSource <-chan int, inUse []*gsync.Mutex, log zerolog.Logger) (network.ConnectionServer, error) {
	localTCP, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	outboundIP, err := getOutboundIP()
	if err != nil {
		return nil, err
	}
	outboundPort := localTCP.Port
	outboundTCP := &net.TCPAddr{IP: net.ParseIP(outboundIP), Port: outboundPort, Zone: ""}

	return &connServer{
		outboundAddr: outboundTCP,
		localAddr:    localTCP,
		remoteAddrs:  remoteAddrs,
		listenChan:   make(chan net.Conn),
		dialChan:     make(chan net.Conn),
		dialSource:   dialSource,
		inUse:        inUse,
		exitChan:     make(chan struct{}),
		log:          log,
	}, nil
}

func (cs *connServer) ListenChannel() <-chan net.Conn {
	return cs.listenChan
}

func (cs *connServer) DialChannel() <-chan net.Conn {
	return cs.dialChan
}

func (cs *connServer) Listen() error {
	ln, err := net.ListenTCP("tcp", cs.outboundAddr)
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
				select {
				case cs.listenChan <- link:
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

func (cs *connServer) StartDialing() {
	cs.wg.Add(1)
	go func() {
		defer close(cs.dialChan)
		defer cs.wg.Done()
		for {
			select {
			case <-cs.exitChan:
				return
			case remotePid, ok := <-cs.dialSource:
				if !ok {
					<-cs.exitChan
					return
				}
				// check if we are already syncing with remotePeer
				m := cs.inUse[remotePid]
				if !m.TryAcquire() {
					continue
				}
				dialer := &net.Dialer{Deadline: time.Now().Add(time.Second * 2)}
				link, err := dialer.Dial("tcp", cs.remoteAddrs[remotePid])
				if err != nil {
					cs.log.Error().Str("where", "connServer.Dial").Msg(err.Error())
					m.Release()
					continue
				}
				select {
				case cs.dialChan <- link:
					cs.log.Info().Msg(logging.ConnectionEstablished)
				default:
					link.Close()
					m.Release()
					cs.log.Info().Msg(logging.TooManyOutgoing)
				}
			}
		}
	}()

}

func (cs *connServer) Stop() {
	close(cs.exitChan)
	cs.wg.Wait()
}

func getOutboundIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		ip := strings.Split(addr.String(), "/")[0]
		parts := strings.Split(ip, ".")
		if len(parts) != 4 || parts[0] == "127" {
			continue
		}
		return ip, nil
	}
	return "", nil
}
