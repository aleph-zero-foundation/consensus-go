// Package rmc implements reliable multicast protocol for disseminating units.
//
// In addition it exchanges signatures, and accepts multisigned units disseminated by other processes.
package rmc

import (
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
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
	pid          uint16
	dag          gomel.Dag
	adder        gomel.Adder
	netserv      network.Server
	fallback     gsync.Fallback
	requests     []chan *request
	waitingUnits chan gomel.Unit
	state        *rmcbox.RMC
	canMulticast sync.Mutex
	wg           sync.WaitGroup
	outPool      gsync.WorkerPool
	inPool       gsync.WorkerPool
	timeout      time.Duration
	log          zerolog.Logger
}

// NewServer returns a server that runs rmc protocol
func NewServer(pid uint16, dag gomel.Dag, adder gomel.Adder, netserv network.Server, state *rmcbox.RMC, timeout time.Duration, log zerolog.Logger) gsync.MulticastServer {
	nProc := int(dag.NProc())
	requests := make([]chan *request, nProc)
	for i := 0; i < nProc; i++ {
		requests[i] = make(chan *request, requestSize)
	}
	waitingUnits := make(chan gomel.Unit, nProc)
	s := &server{
		pid:          pid,
		dag:          dag,
		adder:        adder,
		netserv:      netserv,
		requests:     requests,
		waitingUnits: waitingUnits,
		state:        state,
		timeout:      timeout,
		log:          log,
	}
	s.outPool = gsync.NewPerPidPool(dag.NProc(), outPoolSize, s.out)
	s.inPool = gsync.NewPool(inPoolSize*nProc, s.in)
	return s
}

// Start starts worker pools
func (s *server) Start() {
	s.wg.Add(1)
	go s.translator()
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
	close(s.waitingUnits)
	s.wg.Wait()
}

// The fallback has to check that all the units are multisigned as well.
// RMC guarantees that a process can create only one unit per height.
// There is no guarantee that there are no forks among parents of a signed unit.
func (s *server) SetFallback(fbk gsync.Fallback) {
	s.fallback = fbk
}

func (s *server) Send(unit gomel.Unit) {
	s.waitingUnits <- unit
}

func (s *server) translator() {
	defer s.wg.Done()
	for {
		unit, isOpen := <-s.waitingUnits
		if !isOpen {
			return
		}
		id := unitID(unit, s.dag.NProc())
		data, err := encoding.EncodeUnit(unit)
		if err != nil {
			s.log.Error().Str("where", "rmcServer.Send.EncodeUnit").Msg(err.Error())
			return
		}
		s.canMulticast.Lock()
		for _, i := range rand.Perm(int(s.dag.NProc())) {
			if i == int(s.pid) {
				continue
			}
			s.requests[i] <- newRequest(id, data, sendData)

		}
	}
}
