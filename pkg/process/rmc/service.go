package rmc

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/rmc/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/rmc/multicast"
)

const (
	// Some magic numbers for rmc. All below are ratios, they get multiplied with nProc.
	mcRequestsSize    = 10
	mcAcceptedSize    = 2
	fetchRequestsSize = 1
)

type service struct {
	pid           int
	dag           gomel.Dag
	rs            gomel.RandomSource
	units         chan gomel.Unit
	accepted      chan []byte
	mcRequests    chan multicast.Request
	fetchRequests chan gomel.Preunit
	mcServer      *multicast.Server
	fetchServer   *fetch.Server
	log           zerolog.Logger
}

// NewService creates a new service for rmc.
// It returns the service and a callback for creator to call on unit creation.
func NewService(dag gomel.Dag, rs gomel.RandomSource, config *process.RMC, log zerolog.Logger) (process.Service, gomel.Callback, error) {
	// state contains information about all rmc exchanges
	state := rmc.New(config.Pubs, config.Priv)

	dialer, listener, err := tcp.NewNetwork(config.LocalAddress[0], config.RemoteAddresses[0], log)
	if err != nil {
		return nil, nil, err
	}

	// mcRequests is a channel for requests to multicast
	mcRequests := make(chan multicast.Request, mcRequestsSize*dag.NProc())
	// accepted is a channel for succesfully multicasted data
	accepted := make(chan []byte, mcAcceptedSize*dag.NProc())
	mcServer := multicast.NewServer(uint16(config.Pid), dag.NProc(), state, mcRequests, accepted, dialer, listener, config.Timeout, log)

	dialer, listener, err = tcp.NewNetwork(config.LocalAddress[1], config.RemoteAddresses[1], log)
	if err != nil {
		return nil, nil, err
	}
	// fetchRequests is a channel for preunits which has been succesfully multicasted
	// but we cannot add them to the poset due to uknownParents error.
	fetchRequests := make(chan gomel.Preunit, fetchRequestsSize*dag.NProc())
	fetchServer := fetch.NewServer(uint16(config.Pid), dag, rs, state, fetchRequests, dialer, listener, config.Timeout, log)

	// units is a channel on which create service should send newly created units for rmc
	units := make(chan gomel.Unit)

	return &service{
			dag:           dag,
			rs:            rs,
			pid:           config.Pid,
			units:         units,
			accepted:      accepted,
			mcRequests:    mcRequests,
			mcServer:      mcServer,
			fetchServer:   fetchServer,
			fetchRequests: fetchRequests,
			log:           log,
		}, func(_ gomel.Preunit, unit gomel.Unit, err error) {
			if err != nil {
				return
			}
			units <- unit
		}, nil
}

// getID returns rmc id for a given unit
func getID(u gomel.Unit, nProc int) uint64 {
	return uint64(u.Creator()) + uint64(nProc)*uint64(u.Height())
}

// translator reads units created by the create service
// and creates multicast requests to all other pids.
func (s *service) translator() {
	for {
		unit, ok := <-s.units
		if !ok {
			return
		}
		for i := 0; i < s.dag.NProc(); i++ {
			if i == s.pid {
				continue
			}
			id := getID(unit, s.dag.NProc())
			data, err := rmc.EncodeUnit(unit)
			if err != nil {
				continue
			}
			s.mcRequests <- multicast.NewRequest(id, uint16(i), data, multicast.SendData)
		}
	}
}

// validator validates succesfully multicasted data
func (s *service) validator() {
	for {
		data, ok := <-s.accepted
		if !ok {
			return
		}
		if pu, err := rmc.DecodePreunit(data); pu != nil || err != nil {
			if err != nil {
				continue
			}
			s.dag.AddUnit(pu, s.rs, func(preunit gomel.Preunit, u gomel.Unit, err error) {
				switch err.(type) {
				case *gomel.UnknownParents:
					s.fetchRequests <- pu
				}
			})
		}
	}
}

func (s *service) Start() error {
	go s.translator()
	go s.validator()
	s.mcServer.Start()
	s.fetchServer.Start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.mcServer.StopOut()
	s.fetchServer.StopOut()
	time.Sleep(5 * time.Second)
	s.mcServer.StopIn()
	s.fetchServer.StopIn()
	close(s.units)
	close(s.accepted)
	s.log.Info().Msg(logging.ServiceStopped)
}
