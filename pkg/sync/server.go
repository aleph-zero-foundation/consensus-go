package sync

import (
	"net"
	"sync"

	"github.com/rs/zerolog"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
)

// Server receives ready-to-use incoming connections and establishes outgoing ones,
// to later handle them using the provided protocols.
type Server struct {
	pid          uint16
	poset        gomel.Poset
	inConnChan   <-chan net.Conn
	pidDialChan  <-chan uint16
	dialer       network.Dialer
	inSyncProto  Protocol
	outSyncProto Protocol
	nInitSync    uint
	nRecvSync    uint
	syncIds      []uint32
	inUse        []*mutex
	exitChan     chan struct{}
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewServer constructs a server for the given poset, channels of incoming and outgoing connections, protocols for connection handling,
// and maximal numbers of syncs to initialize and receive.
func NewServer(myPid uint16, poset gomel.Poset, inConnChan <-chan net.Conn, pidDialChan <-chan uint16, dialer network.Dialer, inSyncProto, outSyncProto Protocol, nInitSync, nRecvSync uint, log zerolog.Logger) *Server {
	inUse := make([]*mutex, dialer.Length())
	for i := range inUse {
		inUse[i] = newMutex()
	}
	return &Server{
		pid:          myPid,
		poset:        poset,
		inConnChan:   inConnChan,
		pidDialChan:  pidDialChan,
		dialer:       dialer,
		inSyncProto:  inSyncProto,
		outSyncProto: outSyncProto,
		nInitSync:    nInitSync,
		nRecvSync:    nRecvSync,
		inUse:        inUse,
		syncIds:      make([]uint32, dialer.Length()),
		exitChan:     make(chan struct{}),
		log:          log,
	}
}

// Start starts server
func (s *Server) Start() {
	for i := uint(0); i < s.nRecvSync; i++ {
		s.wg.Add(1)
		go s.inDispatcher()
	}
	for i := uint(0); i < s.nInitSync; i++ {
		s.wg.Add(1)
		go s.outDispatcher()
	}
}

// Stop stops server
func (s *Server) Stop() {
	close(s.exitChan)
	s.wg.Wait()
}

func (s *Server) inDispatcher() {
	defer s.wg.Done()
	for {
		select {
		case <-s.exitChan:
			return
		case link, ok := <-s.inConnChan:
			if !ok {
				<-s.exitChan
				return
			}
			go func() {
				g, err := getGreeting(link)
				if err != nil {
					s.log.Error().Str("where", "syncServer.inDispatcher.greeting").Msg(err.Error())
					link.Close()
					return
				}
				if g.pid >= uint16(len(s.inUse)) {
					s.log.Warn().Uint16(logging.PID, g.pid).Msg("Called by a stranger")
					link.Close()
					return
				}
				m := s.inUse[g.pid]
				if !m.tryAcquire() {
					link.Close()
					return
				}
				log := s.log.With().Uint16(logging.PID, g.pid).Uint32(logging.ISID, g.sid).Logger()
				conn := newConn(link, m, 0, 6, log) // greeting has 6 bytes
				s.inSyncProto.Run(s.poset, conn)
			}()
		}
	}
}

func (s *Server) outDispatcher() {
	defer s.wg.Done()
	for {
		select {
		case <-s.exitChan:
			return
		case remotePid, ok := <-s.pidDialChan:
			if !ok {
				<-s.exitChan
				continue
			}
			go func() {
				m := s.inUse[remotePid]
				if !m.tryAcquire() {
					return
				}
				link, err := s.dialer.Dial(remotePid)
				if err != nil {
					s.log.Error().Str("where", "syncServer.outDispatcher.dial").Msg(err.Error())
					m.release()
					return
				}
				g := &greeting{
					pid: uint16(s.pid),
					sid: s.syncIds[remotePid],
				}
				s.syncIds[remotePid]++
				err = g.send(link)
				if err != nil {
					s.log.Error().Str("where", "syncServer.outDispatcher.greeting").Msg(err.Error())
					m.release()
					link.Close()
					return
				}
				log := s.log.With().Int(logging.PID, int(remotePid)).Uint32(logging.OSID, g.sid).Logger()
				conn := newConn(link, m, 6, 0, log) // greeting has 6 bytes
				s.outSyncProto.Run(s.poset, conn)
			}()
		}
	}
}
