// Package rmc implements reliable multicast protocol for disseminating units.
//
// In addition it exchanges signatures, and accepts multisigned units disseminated by other processes.
package rmc

import (
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/rmcbox"
)

const (
	inPoolSize = 2
)

// server is a multicast server
type server struct {
	pid                 uint16
	nProc               uint16
	orderer             gomel.Orderer
	netserv             network.Server
	state               *rmcbox.RMC
	multicastInProgress sync.Mutex
	inPool              gsync.WorkerPool
	log                 zerolog.Logger
	quit                int64
	mx                  sync.RWMutex
	wg                  sync.WaitGroup
}

// NewServer returns a server that runs rmc protocol
func NewServer(conf config.Config, orderer gomel.Orderer, netserv network.Server, log zerolog.Logger) (gsync.Server, gsync.Multicast) {
	nProc := int(conf.NProc)
	s := &server{
		pid:     conf.Pid,
		nProc:   conf.NProc,
		orderer: orderer,
		netserv: netserv,
		state:   rmcbox.New(conf.RMCPublicKeys, conf.RMCPrivateKey),
		log:     log,
		quit:    0,
	}
	s.inPool = gsync.NewPool(inPoolSize*nProc, s.in)
	config.AddCheck(conf, s.finishedRMC)
	return s, s.send
}

// Start starts worker pools
func (s *server) Start() {
	s.inPool.Start()
}

// StopIn stops incoming connections
func (s *server) StopIn() {
	s.inPool.Stop()
}

// StopOut stops outgoing connections
func (s *server) StopOut() {
	s.mx.Lock()
	defer s.mx.Unlock()
	atomic.StoreInt64(&s.quit, 1)
	s.wg.Wait()
}

func (s *server) send(unit gomel.Unit) {
	s.mx.RLock()
	defer s.mx.RUnlock()
	if s.quit != 0 {
		return
	}
	if unit.Creator() == s.pid {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.multicast(unit)
		}()
	}
}

func (s *server) finishedRMC(u gomel.Unit, _ gomel.Dag) error {
	if u.Creator() == s.pid {
		// We trust our own units.
		return nil
	}
	rmcID := gomel.UnitID(u)
	if s.state.Status(rmcID) != rmcbox.Finished {
		return s.fetchFinished(u, u.Creator())
	}
	pu, err := encoding.DecodePreunit(s.state.Data(rmcID))
	if err != nil {
		return err
	}
	if *pu.Hash() != *u.Hash() {
		return gomel.NewComplianceError(rmcMismatch)
	}
	return nil
}

func (s *server) fetchFinished(u gomel.Unit, source uint16) error {
	conn, err := s.netserv.Dial(source)
	if err != nil {
		return err
	}
	defer conn.Close()
	id := gomel.UnitID(u)
	err = rmcbox.Greet(conn, s.pid, id, requestFinished)
	if err != nil {
		return err
	}
	data, err := s.state.AcceptFinished(id, u.Creator(), conn)
	if err != nil {
		return err
	}
	pu, err := encoding.DecodePreunit(data)
	if err != nil {
		return err
	}
	if *pu.Hash() != *u.Hash() {
		return gomel.NewComplianceError(rmcMismatch)
	}
	return nil
}

const rmcMismatch = "unit differs from successfully RMCd unit"

type unfinishedRMC struct{}

func (e *unfinishedRMC) Error() string {
	return "This instance of RMC is not yet finished"
}
