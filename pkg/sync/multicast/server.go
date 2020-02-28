// Package multicast implements a multicasting service to disseminate units created by us.
//
// It also accepts units multicasted by other processes.
// We might not be able to insert some of these units into our dag if we don't have their parents, so a fallback mechanism is needed.
package multicast

import (
	"math/rand"
	"sync/atomic"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/encoding"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/core-go/pkg/network"
)

const (
	requestSize = 10
	outPoolSize = 4
	inPoolSize  = 2
)

// request represents a request to send the encoded unit to the committee member indicated by pid.
type request struct {
	encUnit []byte
	height  int
}

type server struct {
	conf     config.Config
	orderer  gomel.Orderer
	netserv  network.Server
	requests []chan request
	outPool  sync.WorkerPool
	inPool   sync.WorkerPool
	quit     int64
	log      zerolog.Logger
}

// NewServer returns a server that runs the multicast protocol.
func NewServer(conf config.Config, orderer gomel.Orderer, netserv network.Server, log zerolog.Logger) (sync.Server, sync.Multicast) {
	nProc := int(conf.NProc)
	requests := make([]chan request, nProc)
	for i := 0; i < nProc; i++ {
		requests[i] = make(chan request, requestSize)
	}
	s := &server{
		conf:     conf,
		orderer:  orderer,
		netserv:  netserv,
		requests: requests,
		log:      log,
	}
	s.outPool = sync.NewPerPidPool(conf.NProc, outPoolSize, s.Out)
	s.inPool = sync.NewPool(inPoolSize*nProc, s.In)
	return s, s.send
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	atomic.StoreInt64(&s.quit, 1)
	for i := uint16(0); i < s.conf.NProc; i++ {
		close(s.requests[i])
	}
	s.outPool.Stop()
}

func (s *server) send(unit gomel.Unit) {
	if unit.Creator() != s.conf.Pid {
		panic("Attempting to multicast unit that we didn't create")
	}
	encUnit, err := encoding.EncodeUnit(unit)
	if err != nil {
		s.log.Error().Str("where", "multicastServer.Send.EncodeUnit").Msg(err.Error())
		return
	}
	for _, i := range rand.Perm(int(s.conf.NProc)) {
		if i == int(s.conf.Pid) {
			continue
		}
		if atomic.LoadInt64(&s.quit) == 0 {
			s.requests[i] <- request{encUnit, unit.Height()}
		}
	}
}
