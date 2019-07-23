package multicast

import (
	"bytes"
	"math/rand"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/encoding/custom"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
)

// NewServer returns a server that runs multicast protocol and callback for create service
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, dialer network.Dialer, listener network.Listener, timeout time.Duration, fallback sync.Fallback, log zerolog.Logger) (sync.Server, func(gomel.Unit)) {
	requests := make(chan request, requestsSize*dag.NProc())
	proto := newProtocol(pid, dag, randomSource, dialer, listener, timeout, fallback, requests, log)
	return &server{
			requests: requests,
			outPool:  sync.NewPool(uint(mcOutWPSize*dag.NProc()), proto.Out),
			inPool:   sync.NewPool(uint(mcInWPSize*dag.NProc()), proto.In),
		}, func(unit gomel.Unit) {
			buffer := &bytes.Buffer{}
			encoder := custom.NewEncoder(buffer)
			err := encoder.EncodeUnit(unit)
			if err != nil {
				return
			}
			encUnit := buffer.Bytes()[:]
			for _, i := range rand.Perm(dag.NProc()) {
				if i == int(pid) {
					continue
				}
				requests <- request{encUnit, unit.Height(), uint16(i)}
			}
		}
}

//request represents a request to send the encoded unit to the committee member indicated by pid.
type request struct {
	encUnit []byte
	height  int
	pid     uint16
}

type server struct {
	requests chan<- request
	outPool  *sync.Pool
	inPool   *sync.Pool
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	close(s.requests)
	s.outPool.Stop()
}
