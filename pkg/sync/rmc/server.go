// Package rmc implements reliable multicast protocol for disseminating units.
//
// In addition it exchanges signatures, and accepts multisigned units disseminated by other processes.
package rmc

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	rmcbox "gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

const (
	requestSize = 10
	outPoolSize = 4
	inPoolSize  = 2
)

// server is a multicast server
type server struct {
	pid          uint16
	dag          gomel.Dag
	randomSource gomel.RandomSource
	netserv      network.Server
	fallback     sync.QueryServer
	requests     []chan *request
	state        *rmcbox.RMC
	outPool      sync.WorkerPool
	inPool       sync.WorkerPool
	timeout      time.Duration
	log          zerolog.Logger
}

// NewServer returns a server that runs rmc protocol
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, netserv network.Server, state *rmcbox.RMC, timeout time.Duration, log zerolog.Logger) sync.MulticastServer {
	nProc := int(dag.NProc())
	requests := make([]chan *request, nProc)
	for i := 0; i < nProc; i++ {
		requests[i] = make(chan *request, requestSize)
	}
	s := &server{
		pid:          pid,
		dag:          dag,
		randomSource: randomSource,
		netserv:      netserv,
		requests:     requests,
		state:        state,
		timeout:      timeout,
		log:          log,
	}
	s.outPool = sync.NewPerPidPool(dag.NProc(), outPoolSize, s.out)
	s.inPool = sync.NewPool(inPoolSize*nProc, s.in)
	return s
}

// Start starts worker pools
func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

// StopIn stops incoming connections
func (s *server) StopIn() {
	s.inPool.Stop()
}

// StopOut stops outgoing connections
func (s *server) StopOut() {
	nProc := int(s.dag.NProc())
	for i := 0; i < nProc; i++ {
		close(s.requests[i])
	}
	s.outPool.Stop()
}

func (s *server) SetFallback(qs sync.QueryServer) {
	s.fallback = qs
}

func (s *server) Send(unit gomel.Unit) {
	id := unitID(unit, s.dag.NProc())
	data, err := custom.EncodeUnit(unit)
	if err != nil {
		s.log.Error().Str("where", "rmcServer.Send.EncodeUnit").Msg(err.Error())
		return
	}
	for _, i := range rand.Perm(int(s.dag.NProc())) {
		if i == int(s.pid) {
			continue
		}
		s.requests[i] <- newRequest(id, data, sendData)
	}
}
