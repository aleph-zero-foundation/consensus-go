package sync

import (
	"net"
	"sync"

	"github.com/rs/zerolog"
	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
)

// Server retrieves ready-to-use connections and dispatches workers that use
// the connections for running in/out synchronizations according to a sync-protocol
type Server struct {
	pid          uint16
	poset        gomel.Poset
	inConnChan   <-chan net.Conn
	outConnChan  <-chan net.Conn
	inSyncProto  Protocol
	outSyncProto Protocol
	nInitSync    uint
	nRecvSync    uint
	addrIds      map[string]uint16
	syncIds      []uint32
	inUse        []*Mutex
	exitChan     chan struct{}
	wg           sync.WaitGroup
	log          zerolog.Logger
}

// NewServer constructs a server for the given poset, channels of incoming and outgoing connections, protocols for connection handling,
// and maximal numbers of syncs to initialize and receive.
func NewServer(myPid uint16, poset gomel.Poset, inConnChan, outConnChan <-chan net.Conn, inSyncProto, outSyncProto Protocol, nInitSync, nRecvSync uint, remoteAddrs []string, inUse []*Mutex, log zerolog.Logger) *Server {
	addrIds := map[string]uint16{}
	for pid, addr := range remoteAddrs {
		addrIds[addr] = uint16(pid)
	}
	return &Server{
		pid:          myPid,
		poset:        poset,
		inConnChan:   inConnChan,
		outConnChan:  outConnChan,
		inSyncProto:  inSyncProto,
		outSyncProto: outSyncProto,
		nInitSync:    nInitSync,
		nRecvSync:    nRecvSync,
		addrIds:      addrIds,
		inUse:        inUse,
		syncIds:      make([]uint32, len(remoteAddrs)),
		exitChan:     make(chan struct{}),
		log:          log,
	}
}

// Start starts server
// THIS REQUIRES A SERIOUS REFACTOR -- WE DO NOT NEED TO SPAWN GOROUTINES HERE IN ADVANCE
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
			g, err := getGreeting(link)
			if err != nil {
				s.log.Error().Str("where", "syncServer.inDispatcher.greeting").Msg(err.Error())
				link.Close()
				continue
			}
			if g.pid >= uint16(len(s.inUse)) {
				s.log.Warn().Uint16(logging.PID, g.pid).Msg("Called by a stranger")
				link.Close()
				continue
			}
			m := s.inUse[g.pid]
			if !m.TryAcquire() {
				link.Close()
				continue
			}
			log := s.log.With().Uint16(logging.PID, g.pid).Uint32(logging.ISID, g.sid).Logger()
			conn := newConn(link, m, 0, 6, log) // greeting has 6 bytes
			s.inSyncProto.Run(s.poset, conn)
		}
	}
}

func (s *Server) outDispatcher() {
	defer s.wg.Done()
	for {
		select {
		case <-s.exitChan:
			return
		case link, ok := <-s.outConnChan:
			if !ok {
				<-s.exitChan
				return
			}
			remotePid, ok := s.addrIds[link.RemoteAddr().String()]
			if !ok {
				// TODO: handle error.
				// Alternatively and better -- ensure this is done in a way that actually is guaranteed to work.
				// The strings returned from net.Addr only guarantee the ability to be used to create a similar Addr object.
				// No guarantees for format. Seems to be working for now though.
				continue
			}
			m := s.inUse[remotePid]
			g := &greeting{
				pid: uint16(s.pid),
				sid: s.syncIds[remotePid],
			}
			s.syncIds[remotePid]++
			err := g.send(link)
			if err != nil {
				m.Release()
				continue
			}
			log := s.log.With().Int(logging.PID, int(remotePid)).Uint32(logging.OSID, g.sid).Logger()
			conn := newConn(link, m, 6, 0, log) // greeting has 6 bytes
			s.outSyncProto.Run(s.poset, conn)
		}
	}
}
