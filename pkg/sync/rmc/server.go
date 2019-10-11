// Package rmc implements reliable multicast protocol for disseminating units.
//
// In addition it exchanges signatures, and accepts multisigned units disseminated by other processes.
package rmc

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	rmcbox "gitlab.com/alephledger/consensus-go/pkg/rmc"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
)

const (
	requestSize = 10
	outPoolSize = 4
	inPoolSize  = 2
)

// server is a multicast server
type server struct {
	pid                 uint16
	dag                 gomel.Dag
	adder               gomel.Adder
	netserv             network.Server
	fallback            gsync.Fallback
	state               *rmcbox.RMC
	multicastInProgress sync.Mutex
	inPool              gsync.WorkerPool
	timeout             time.Duration
	log                 zerolog.Logger
	quit                int64
}

// NewServer returns a server that runs rmc protocol
func NewServer(pid uint16, dag gomel.Dag, adder gomel.Adder, netserv network.Server, state *rmcbox.RMC, timeout time.Duration, log zerolog.Logger) gsync.MulticastServer {
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

// The fallback has to check that all the units are multisigned as well.
// RMC guarantees that a process can create only one unit per height.
// There is no guarantee that there are no forks among parents of a signed unit.
func (s *server) SetFallback(fbk gsync.Fallback) {
	s.fallback = fbk
}

func (s *server) Send(unit gomel.Unit) {
	go s.multicast(unit)
}
