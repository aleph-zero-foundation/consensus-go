package rmc

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
)

type service struct {
	dag              gomel.Dag
	rs               gomel.RandomSource
	pid              int
	requests         <-chan []byte
	accepted         chan []byte
	internalRequests chan rmc.Request
	server           *rmc.Server
	log              zerolog.Logger
}

// NewService creates a new service for rmc
func NewService(dag gomel.Dag, rs gomel.RandomSource, requests <-chan []byte, config process.RMC, log zerolog.Logger) (process.Service, error) {
	dialer, listener, err := tcp.NewNetwork(config.LocalAddress, config.RemoteAddresses, log)
	if err != nil {
		return nil, err
	}
	internalRequests := make(chan rmc.Request)
	accepted := make(chan []byte)
	server := rmc.NewServer(uint16(config.Pid), config.Pubs, config.Priv, internalRequests, accepted, dialer, listener, config.Timeout, log)

	return &service{
		dag:              dag,
		rs:               rs,
		pid:              config.Pid,
		requests:         requests,
		accepted:         accepted,
		internalRequests: internalRequests,
		server:           server,
		log:              log,
	}, nil
}

func (s *service) translator() {
	id := uint64(s.pid)
	for {
		data, ok := <-s.requests
		if !ok {
			return
		}
		for i := 0; i < s.dag.NProc(); i++ {
			if i == s.pid {
				continue
			}
			s.internalRequests <- rmc.NewRequest(rmc.SendData, id, uint16(i), data)
		}
		id += uint64(s.dag.NProc())
	}
}

func (s *service) validator() {
	for {
		data, ok := <-s.accepted
		if !ok {
			return
		}
		if u, err := rmc.DecodeUnit(data); u != nil || err != nil {
			// do sth with the unit or handle the error
			if err != nil {
				continue
			}
			s.dag.AddUnit(u, s.rs, gomel.NopCallback)
		}
	}
}

func (s *service) Start() error {
	go s.translator()
	go s.validator()
	s.server.Start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.server.StopOut()
	time.Sleep(5 * time.Second)
	s.server.StopIn()
	s.log.Info().Msg(logging.ServiceStopped)
}
