// Package rmc implements reliable multicast protocol for disseminating units.
//
// In addition it exchanges signatures, and accepts multisigned units disseminated by other processes.
package rmc

import (
	"fmt"
	"math/rand"
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
		pu, err = s.fetchFinishedFromAll(u)
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

func hashToInt64(hash gomel.Hash) int64 {
	var result int64
	for p, v := range hash[:] {
		result += int64(v) * (1 << p)
	}
	return result
}

func (s *server) fetchFinishedFromAll(u gomel.Unit) (gomel.Preunit, error) {
	pu, err := s.fetchFinished(u, u.Creator())
	if err != nil {
		s.log.Error().Str("where", "rmc.fetchFinishedFromAll.callForPid").Msg(err.Error())
	}
	// call all other nodes in random order
	rand := rand.New(rand.NewSource(hashToInt64(*u.Hash())))
	for _, pidi := range rand.Perm(int(s.nProc)) {
		if pu != nil {
			break
		}
		pid := uint16(pidi)
		if pid == s.pid || pid == u.Creator() {
			continue
		}
		pu, err = s.fetchFinished(u, pid)
		if err != nil {
			s.log.Error().Str("where", "rmc.fetchFinishedFromAll.callForPid").Msg(err.Error())
			continue
		}
	}
	if pu == nil {
		return nil, fmt.Errorf(
			"rmc.fetchFinishedFromAll: unable to fetch a finished unit (creator=%d, height=%d, hash=%v)",
			u.Creator(), u.Height(), *u.Hash())
	}
	return pu, nil
}

func (s *server) fetchFinished(u gomel.Unit, pid uint16) (gomel.Preunit, error) {
	conn, err := s.netserv.Dial(pid)
	if err != nil {
		return nil, fmt.Errorf("rmc.fetchFinishedFromAll.Dial for PID=%d: %v", pid, err)
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			s.log.Error().Str("where", "rmc.fetchFinishedFromAll.Close").Msg(fmt.Sprintf("error while closing connection for PID=%d: %v", pid, err))
		}
	}()

	id := gomel.UnitID(u)
	err = rmcbox.Greet(conn, s.pid, id, msgRequestFinished)
	if err != nil {
		return nil, fmt.Errorf("rmc.fetchFinished.Greet for PID=%d: %v", pid, err)
	}
	err = conn.Flush()
	if err != nil {
		return nil, fmt.Errorf("rmc.fetchFinished.Flush for PID=%d: %v", pid, err)
	}
	data, err := s.state.AcceptFinished(id, u.Creator(), conn)
	if err != nil {
		return nil, fmt.Errorf("rmc.fetchFinished.AcceptFinished for PID=%d: %v", pid, err)
	}
	pu, err := encoding.DecodePreunit(data)
	if err != nil {
		return nil, fmt.Errorf("rmc.fetchFinished.DecodePreunit for PID=%d: %v", pid, err)
	}
	return pu, nil
}
