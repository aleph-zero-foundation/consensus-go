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
	callers     []*link
	receivers   []*link
	queue       chan network.Connection
	tcpListener *net.TCPListener
	mx          []sync.Mutex
	wg          sync.WaitGroup
	quit        int32
	log         zerolog.Logger
}

// NewServer initializes network setup for the given local address and the set of remote addresses.
// Returns an object the implements BOTH network.Server and process.Service interface.
// It needs to be started as a service to activate listening for incoming TCP connections.
func NewServer(localAddress string, remoteAddresses []string, log zerolog.Logger) (network.Server, error) {
	nProc := len(remoteAddresses)
	s := &server{
		localAddr:   localAddress,
		remoteAddrs: remoteAddresses,
		callers:     make([]*link, nProc),
		receivers:   make([]*link, 0, nProc),
		queue:       make(chan network.Connection, 5*nProc),
		mx:          make([]sync.Mutex, nProc),
		log:         log,
	}
	return s, nil
}

func (s *server) Dial(pid uint16, timeout time.Duration) (network.Connection, error) {
	caller, err := s.getCaller(pid, timeout)
	if err != nil {
		return nil, err
	}
	return caller.call(), nil
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
		for atomic.LoadInt32(&s.quit) == 0 {
			ln, err := s.tcpListener.Accept()
			if err != nil {
				continue
			}
			newLink := newLink(ln, s.queue, &s.wg, &s.quit, s.log)
			s.receivers = append(s.receivers, newLink)
			newLink.start()
		}
	}()
	return nil
}

func (s *server) Stop() {
	atomic.StoreInt32(&s.quit, 1)
	for _, link := range s.callers {
		if link != nil {
			link.stop()
		}
	}
	for _, link := range s.receivers {
		link.stop()
	}
	s.tcpListener.Close()
	s.wg.Wait()
}

func (s *server) getCaller(pid uint16, timeout time.Duration) (*link, error) {
	s.mx[pid].Lock()
	defer s.mx[pid].Unlock()
	if s.callers[pid] == nil || s.callers[pid].isDead() {
		ln, err := net.DialTimeout("tcp", s.remoteAddrs[pid], timeout)
		if err != nil {
			return nil, err
		}
		newLink := newLink(ln, nil, &s.wg, &s.quit, s.log)
		s.callers[pid] = newLink
		newLink.start()
	}
	return s.callers[pid], nil
}
