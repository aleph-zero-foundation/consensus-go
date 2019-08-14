package persistent

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

type server struct {
	localAddr   string
	remoteAddrs []string
	dialers     []*dialer
	listeners   []*listener
	queue       chan network.Connection
	tcpListener *net.TCPListener
	mx          sync.Mutex
	wg          sync.WaitGroup
	quit        int32
	log         zerolog.Logger
}

// NewServer initializes network setup for the given local address and the set of remote addresses.
func NewServer(localAddress string, remoteAddresses []string, log zerolog.Logger) (network.Server, error) {
	nProc := len(remoteAddresses)
	s := &server{
		localAddr:   localAddress,
		remoteAddrs: remoteAddresses,
		dialers:     make([]*dialer, nProc),
		listeners:   make([]*listener, 0, nProc),
		queue:       make(chan network.Connection, 5*nProc),
		log:         log,
	}
	return s, nil
}

func (s *server) Dial(pid uint16, timeout time.Duration) (network.Connection, error) {
	s.mx.Lock()
	if s.dialers[pid] == nil {
		nd, err := newDialer(s.remoteAddrs[pid], timeout, &s.wg, &s.quit, s.log)
		if err != nil {
			s.mx.Unlock()
			return nil, err
		}
		s.dialers[pid] = nd
		nd.start()
	}
	s.mx.Unlock()
	return s.dialers[pid].dial(), nil
}

func (s *server) Listen(timeout time.Duration) (network.Connection, error) {
	select {
	case conn := <-s.queue:
		return conn, nil
	case <-time.After(timeout):
		return nil, errors.New("Listen timed out")
	}
}

func (s *server) Start() error {
	localTCP, err := net.ResolveTCPAddr("tcp", s.localAddr)
	if err != nil {
		return err
	}
	s.tcpListener, err = net.ListenTCP("tcp", localTCP)
	if err != nil {
		return err
	}

	go func() {
		s.wg.Add(1)
		defer s.wg.Done()
		for {
			if atomic.LoadInt32(&s.quit) > 0 {
				return
			}
			link, err := s.tcpListener.Accept()
			if err != nil {
				continue
			}
			nl := newListener(link, s.queue, &s.wg, &s.quit, s.log)
			s.listeners = append(s.listeners, nl)
			nl.start()
		}
	}()

	return nil

}

func (s *server) Stop() {
	atomic.StoreInt32(&s.quit, 1)
	for _, d := range s.dialers {
		if d != nil {
			d.stop()
		}
	}
	s.tcpListener.Close()
	s.wg.Wait()
}
