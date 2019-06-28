package sync

import (
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
	peerSource   *dialer
	inConnChan   <-chan network.Connection
	dialer       network.Dialer
	inSyncProto  Protocol
	outSyncProto Protocol
	nOutSync     uint
	nInSync      uint
	syncIds      []uint32
	inUse        []*mutex
	exitChanIn   chan struct{}
	exitChanOut  chan struct{}
	wgIn         sync.WaitGroup
	wgOut        sync.WaitGroup
	log          zerolog.Logger
}

// NewServer constructs a server for the given poset, channels of incoming and outgoing connections, protocols for connection handling,
// and maximal numbers of syncs to initialize and receive.
func NewServer(myPid uint16, poset gomel.Poset, inConnChan <-chan network.Connection, dialer network.Dialer, inSyncProto, outSyncProto Protocol, nOutSync, nInSync uint, log zerolog.Logger) *Server {
	nProc := uint16(dialer.Length())
	peerSource := newDialer(nProc, myPid)
	inUse := make([]*mutex, nProc)
	for i := range inUse {
		inUse[i] = newMutex()
	}
	return &Server{
		pid:          myPid,
		poset:        poset,
		inConnChan:   inConnChan,
		peerSource:   peerSource,
		dialer:       dialer,
		inSyncProto:  inSyncProto,
		outSyncProto: outSyncProto,
		nOutSync:     nOutSync,
		nInSync:      nInSync,
		inUse:        inUse,
		syncIds:      make([]uint32, dialer.Length()),
		exitChanIn:   make(chan struct{}),
		exitChanOut:  make(chan struct{}),
		log:          log,
	}
}

// Start starts server
func (s *Server) Start() {
	s.wgIn.Add(int(s.nInSync))
	for i := uint(0); i < s.nInSync; i++ {
		go s.inDispatcher()
	}
	s.wgOut.Add(int(s.nOutSync))
	for i := uint(0); i < s.nOutSync; i++ {
		go s.outDispatcher()
	}
}

// StopIn stops handling incoming synchronizations
func (s *Server) StopIn() {
	close(s.exitChanIn)
	s.wgIn.Wait()
}

// StopOut stops handling outgoing synchronizations
func (s *Server) StopOut() {
	close(s.exitChanOut)
	s.wgOut.Wait()
}

func (s *Server) inDispatcher() {
	defer s.wgIn.Done()
	for {
		select {
		case <-s.exitChanIn:
			return
		case link, ok := <-s.inConnChan:
			if !ok {
				<-s.exitChanIn
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
			log := s.log.With().Uint16(logging.PID, g.pid).Uint32(logging.ISID, g.sid).Logger()
			link.SetLogger(log)
			s.inSyncProto.Run(s.poset, link)
		}
	}
}

func (s *Server) outDispatcher() {
	defer s.wgOut.Done()
	for {
		select {
		case <-s.exitChanOut:
			return
		default:
			remotePid := s.peerSource.nextPeer()

			link, err := s.dialer.Dial(remotePid)
			if err != nil {
				s.log.Error().Str("where", "syncServer.outDispatcher.dial").Msg(err.Error())
				continue
			}
			g := &greeting{
				pid: uint16(s.pid),
				sid: s.syncIds[remotePid],
			}
			s.syncIds[remotePid]++
			err = g.send(link)
			if err != nil {
				s.log.Error().Str("where", "syncServer.outDispatcher.greeting").Msg(err.Error())
				link.Close()
				continue
			}
			log := s.log.With().Int(logging.PID, int(remotePid)).Uint32(logging.OSID, g.sid).Logger()
			link.SetLogger(log)
			s.outSyncProto.Run(s.poset, link)
		}
	}
}
