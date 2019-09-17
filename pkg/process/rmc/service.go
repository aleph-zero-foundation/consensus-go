package rmc

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/rmc/multicast"
	gsync "gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fallback"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
)

// Some magic numbers for rmc. All below are ratios, they get multiplied with nProc.
const (
	mcRequestsSize    = 10
	mcAcceptedSize    = 2
	fetchRequestsSize = 1
)

type service struct {
	pid          uint16
	dag          gomel.Dag
	rs           gomel.RandomSource
	units        <-chan gomel.Unit
	accepted     chan []byte
	mcRequests   chan *multicast.Request
	mcServer     *multicast.Server
	canMulticast *sync.Mutex
	fetchServer  gsync.Server
	fallback     gsync.Fallback
	log          zerolog.Logger
	exitChan     chan struct{}
	wg           sync.WaitGroup
}

// NewService creates a new service for rmc.
// It returns the service and a unit channel
// for creator to send newly created units.
func NewService(dag gomel.Dag, rs gomel.RandomSource, config *process.RMC, log zerolog.Logger) (process.Service, chan<- gomel.Unit, error) {
	// state contains information about all rmc exchanges
	state := rmc.New(config.Pubs, config.Priv)

	netserv, err := tcp.NewServer(config.LocalAddress[0], config.RemoteAddresses[0], log)
	if err != nil {
		return nil, nil, err
	}

	// mcRequests is a channel for requests to multicast
	mcRequests := make(chan *multicast.Request, mcRequestsSize*dag.NProc())
	// accepted is a channel for succesfully multicasted data
	accepted := make(chan []byte, mcAcceptedSize*dag.NProc())
	canMulticast := new(sync.Mutex)
	mcServer := multicast.NewServer(uint16(config.Pid), dag, state, mcRequests, canMulticast, accepted, netserv, config.Timeout, log)

	netserv, err = tcp.NewServer(config.LocalAddress[1], config.RemoteAddresses[1], log)
	if err != nil {
		return nil, nil, err
	}

	fetchRequests := make(chan fetch.Request, dag.NProc())
	fbk := fallback.NewFetch(dag, fetchRequests)
	fetchServer := fetch.NewServer(uint16(config.Pid), dag, rs, fetchRequests, netserv, gomel.NopCallback, config.Timeout, fbk, log, 1, 1)

	// units is a channel on which create service should send newly created units for rmc
	units := make(chan gomel.Unit)

	return &service{
			dag:          dag,
			rs:           rs,
			pid:          config.Pid,
			units:        units,
			accepted:     accepted,
			mcRequests:   mcRequests,
			mcServer:     mcServer,
			fetchServer:  fetchServer,
			canMulticast: canMulticast,
			fallback:     fbk,
			log:          log,
			exitChan:     make(chan struct{}),
		},
		units,
		nil
}

// translator reads units created by the create service
// and creates multicast requests to all other pids.
func (s *service) translator() {
	defer s.wg.Done()
	for {
		select {
		case unit, isOpen := <-s.units:
			if !isOpen {
				return
			}
			s.canMulticast.Lock()
			for pid := uint16(0); pid < s.dag.NProc(); pid++ {
				if pid == s.pid {
					continue
				}
				req, err := multicast.NewUnitSendRequest(unit, pid, s.dag.NProc())
				if err != nil {
					s.log.Error().Str("where", "multicast.NewUnitSendRequest").Msg(err.Error())
					continue
				}
				s.mcRequests <- req
			}
		case <-s.exitChan:
			return
		}
	}
}

// validator validates succesfully multicasted data
func (s *service) validator() {
	defer s.wg.Done()
	for {
		select {
		case data, isOpen := <-s.accepted:
			if !isOpen {
				return
			}
			if pu, err := multicast.DecodePreunit(data); pu != nil || err != nil {
				if err != nil {
					s.log.Error().Str("where", "multicast.DecodePreunit").Msg(err.Error())
					continue
				}
				s.dag.AddUnit(pu, s.rs, func(preunit gomel.Preunit, u gomel.Unit, err error) {
					if err != nil {
						switch err.(type) {
						case *gomel.UnknownParents:
							s.fallback.Run(pu)
						}
					}
				})
			}
		case <-s.exitChan:
			return
		}
	}
}

func (s *service) Start() error {
	s.wg.Add(2)
	go s.translator()
	go s.validator()
	s.fetchServer.Start()
	s.mcServer.Start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.mcServer.StopOut()
	s.fetchServer.StopOut()
	time.Sleep(5 * time.Second)
	s.mcServer.StopIn()
	s.fetchServer.StopIn()
	close(s.accepted)
	close(s.exitChan)
	s.wg.Wait()
	s.log.Info().Msg(logging.ServiceStopped)
}
