// Package rmc implements reliable multicast protocol for disseminating units.
//
// In addition it exchanges signatures, and accepts multisigned units disseminated by other processes.
package rmc

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	rmcbox "gitlab.com/alephledger/consensus-go/pkg/rmc"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

const (
	inPoolSize = 2
)

// server is a multicast server
type server struct {
	pid                 uint16
	dag                 gomel.Dag
	adder               gomel.Adder
	netserv             network.Server
	state               *rmcbox.RMC
	multicastInProgress sync.Mutex
	inPool              gsync.WorkerPool
	timeout             time.Duration
	log                 zerolog.Logger
	quit                int64
}

// NewServer returns a server that runs rmc protocol
func NewServer(pid uint16, dag gomel.Dag, adder gomel.Adder, netserv network.Server, state *rmcbox.RMC, timeout time.Duration, log zerolog.Logger) gsync.Server {
	nProc := int(dag.NProc())
	s := &server{
		pid:     pid,
		dag:     dag,
		adder:   adder,
		netserv: netserv,
		state:   state,
		timeout: timeout,
		log:     log,
		quit:    0,
	}
	s.inPool = gsync.NewPool(inPoolSize*nProc, s.in)
	dag.AddCheck(s.finishedRMC)
	dag.AfterInsert(s.send)
	return s
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
	atomic.StoreInt64(&s.quit, 1)
}

func (s *server) send(unit gomel.Unit) {
	if unit.Creator() == s.pid {
		go s.multicast(unit)
	}
}

func (s *server) finishedRMC(u gomel.Unit) error {
	if u.Creator() == s.pid {
		// We trust our own units.
		return nil
	}
	rmcID := gomel.UnitID(u)
	if s.state.Status(rmcID) != rmcbox.Finished {
		return &unfinishedRMC{}
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
	conn, err := s.netserv.Dial(source, s.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.TimeoutAfter(s.timeout)
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

func (s *server) checkErrorHandler(u gomel.Unit, err error, source uint16) error {
	switch err.(type) {
	case *unfinishedRMC:
		return s.fetchFinished(u, source)
	default:
		return err
	}
}

const rmcMismatch = "unit differs from successfully RMCd unit"

type unfinishedRMC struct{}

func (e *unfinishedRMC) Error() string {
	return "This instance of RMC is not yet finished"
}
