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
func NewServer(pid uint16, dag gomel.Dag, randomSource gomel.RandomSource, dialer network.Dialer, listener network.Listener, callback gomel.Callback, timeout time.Duration, fallback sync.Fallback, log zerolog.Logger) (sync.Server, gomel.Callback) {
	requests := make(chan request, requestsSize*dag.NProc())
	proto := newProtocol(pid, dag, randomSource, requests, dialer, listener, callback, timeout, fallback, log)
	return &server{
			requests: requests,
			fallback: fallback,
			outPool:  sync.NewPool(uint(mcOutWPSize*dag.NProc()), proto.Out),
			inPool:   sync.NewPool(uint(mcInWPSize*dag.NProc()), proto.In),
		}, func(_ gomel.Preunit, unit gomel.Unit, err error) {
			if err == nil {
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
}

//request represents a request to send the encoded unit to the committee member indicated by pid.
type request struct {
	encUnit []byte
	height  int
	pid     uint16
}

type server struct {
	requests chan<- request
	fallback sync.Fallback
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
