// Package rmc implements reliable multicast protocol for disseminating units.
//
// In addition it exchanges signatures, and accepts multisigned units disseminated by other processes.
package rmc

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/rmcbox"
)

const (
	inPoolSize  = 8
	rmcMismatch = "unit differs from successfully RMCd unit"
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
	quit                bool
	mx                  sync.RWMutex
	wg                  sync.WaitGroup
}

// NewServer returns a server that runs rmc protocol
func NewServer(conf config.Config, orderer gomel.Orderer, netserv network.Server, log zerolog.Logger) (core.Service, gsync.Multicast) {
	nProc := int(conf.NProc)
	s := &server{
		pid:     conf.Pid,
		nProc:   conf.NProc,
		orderer: orderer,
		netserv: netserv,
		state:   rmcbox.New(conf.RMCPublicKeys, conf.RMCPrivateKey),
		log:     log,
	}
	s.inPool = gsync.NewPool(inPoolSize*nProc, s.in)
	config.AddCheck(conf, s.finishedRMC)
	return s, s.send
}

// Start starts worker pools
func (s *server) Start() error {
	s.inPool.Start()
	return nil
}

// StopIn stops incoming connections
func (s *server) StopIn() {
}

// StopOut stops outgoing connections
func (s *server) Stop() {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.quit = true
	s.wg.Wait()
	s.inPool.Stop()
}

func (s *server) send(unit gomel.Unit) {
	s.mx.RLock()
	defer s.mx.RUnlock()
	if s.quit {
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
	var pu gomel.Preunit
	var err error
	if s.state.Status(rmcID) != rmcbox.Finished {
		pu, err = s.fetchFinishedFromQuorum(u)
	} else {
		pu, err = encoding.DecodePreunit(s.state.Data(rmcID))
	}
	if err != nil {
		return err
	}
	if !gomel.Equal(pu, u) {
		return gomel.NewComplianceError(rmcMismatch)
	}
	return nil
}

func fetchProof(u gomel.Unit) error {
}

func (s *server) fetchFinishedFromQuorum(u gomel.Unit) (gomel.Preunit, error) {
	var wg sync.WaitGroup
	finished := make(chan struct{})
	asked := s.nProc - 1
	semaphore := make(chan struct{}, asked)
	result := make(chan gomel.Preunit, 1)
	for pid := uint16(0); pid < s.nProc; pid++ {
		if pid == s.pid {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			var finishedPu gomel.Preunit
			defer func() {
				result <- finishedPu
			}()

			select {
			case semaphore <- struct{}{}:
			case <-finished:
				return
			default:
				return
			}
			conn, err := s.netserv.Dial(source)
			if err != nil {
				log.Error().Str("where", "rmc.fetchFinishedFromQuorum.Dial").Msg(err.Error())
				return
			}
			defer func() {
				err := conn.Close()
				if err != nil {
					log.Error().Str("where", "rmc.fetchFinishedFromQuorum.Close").Msg(err.Error())
				}
			}()
			select {
			case <-finished:
				return
			default:
			}
			pu, err := s.fetchFinished(u, conn)
			if err != nil {
				log.Error().Str("where", "rmc.fetchFinishedFromQuorum.fetchFinished").Msg(err.Error())
				return
			}
			finishedPu = pu
		}()
	}
	var finishedPu gomel.Preunit
	for count := uint16(0); count < asked; count++ {
		pu := <-result
		if pu != nil && finishedPu == nil {
			finishedPu = pu
		}
	}
	close(finished)
	wg.Wait()
	if finishedPu != nil {
		return finishedPu, nil
	}
	return nil, fmt.Errorf("rmc.fetchFinishedFromQuorum: unable to fetch a finished unit (creator=%d, height=%d, hash=%v)", u.Creator(), u.Height(), *u.Hash())
}

func (s *server) fetchFinished(u gomel.Unit, conn network.Connection) (gomel.Preunit, error) {
	id := gomel.UnitID(u)
	conn, err := s.netserv.Dial(source)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = rmcbox.Greet(conn, s.pid, id, msgRequestFinished)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return nil, err
	}
	data, err := s.state.AcceptFinished(id, u.Creator(), conn)
	if err != nil {
		return nil, err
	}
	pu, err := encoding.DecodePreunit(data)
	if err != nil {
		return nil, err
	}
	return pu, nil
}
