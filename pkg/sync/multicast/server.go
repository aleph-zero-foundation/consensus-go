// Package multicast implements a multicasting service to disseminate units created by us.
//
// It also accepts units multicasted by other processes.
// We might not be able to insert some of these units into our dag if we don't have their parents, so a fallback mechanism is needed.
package multicast

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

const (
	requestSize = 10
	outPoolSize = 4
	inPoolSize  = 2
)

//request represents a request to send the encoded unit to the committee member indicated by pid.
type request struct {
	encUnit []byte
	height  int
}

type server struct {
	pid          uint16
	dag          gomel.Dag
	randomSource gomel.RandomSource
	netserv      network.Server
	fallback     sync.QueryServer
	requests     []chan request
	outPool      sync.WorkerPool
	inPool       sync.WorkerPool
	timeout      time.Duration
	log          zerolog.Logger
}

// NewServer returns a server that runs the multicast protocol.
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, netserv network.Server, timeout time.Duration, log zerolog.Logger) sync.MulticastServer {
	nProc := int(dag.NProc())
	requests := make([]chan request, nProc)
	for i := 0; i < nProc; i++ {
		requests[i] = make(chan request, requestSize)
	}
	s := &server{
		pid:          pid,
		dag:          dag,
		randomSource: randomSource,
		netserv:      netserv,
		requests:     requests,
		timeout:      timeout,
		log:          log,
	}

	s.outPool = sync.NewPerPidPool(dag.NProc(), outPoolSize, s.out)
	s.inPool = sync.NewPool(inPoolSize*nProc, s.in)
	return s

}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

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
	encUnit, err := custom.EncodeUnit(unit)
	if err != nil {
		s.log.Error().Str("where", "multicastServer.Send.EncodeUnit").Msg(err.Error())
		return
	}
	for _, i := range rand.Perm(int(s.dag.NProc())) {
		if i == int(s.pid) {
			continue
		}
		s.requests[i] <- request{encUnit, unit.Height()}
	}
}
