// Package sync implements a service that creates and runs all the necessary syncing servers.
package sync

import (
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/persistent"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/network/udp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	rmcbox "gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
	"gitlab.com/alephledger/consensus-go/pkg/sync/retrying"
	"gitlab.com/alephledger/consensus-go/pkg/sync/rmc"
)

type service struct {
	fallbacks   map[string]sync.Fallback
	servers     map[string]sync.Server
	mcServer    sync.MulticastServer
	subservices []process.Service
	log         zerolog.Logger
}

// NewService creates a new syncing service and the function for multicasting units.
// Each config entry corresponds to a separate sync.Server.
// The returned function should be called on units created by this process after they are added to the poset.
func NewService(dag gomel.Dag, adder gomel.Adder, configs []*process.Sync, log zerolog.Logger) (process.Service, func(gomel.Unit), error) {
	if err := valid(configs); err != nil {
		return nil, nil, err
	}
	pid := configs[0].Pid
	s := &service{
		fallbacks: make(map[string]sync.Fallback),
		servers:   make(map[string]sync.Server),
		log:       log.With().Int(logging.Service, logging.SyncService).Logger(),
	}

	for _, c := range configs {
		var netserv network.Server

		timeout, err := time.ParseDuration(c.Params["timeout"])
		if err != nil {
			return nil, nil, err
		}

		switch c.Type {
		case "multicast":
			lg := log.With().Int(logging.Service, logging.MCService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			server := multicast.NewServer(pid, dag, adder, netserv, timeout, log)
			s.mcServer = server
			s.servers[c.Type] = server

		case "rmc":
			lg := log.With().Int(logging.Service, logging.RMCService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			server := rmc.NewServer(pid, dag, adder, netserv, rmcbox.New(c.Pubs, c.Priv), timeout, lg)
			s.mcServer = server
			s.servers[c.Type] = server

		case "gossip":
			lg := log.With().Int(logging.Service, logging.GossipService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			s.servers[c.Type], s.fallbacks[c.Type] = gossip.NewServer(pid, dag, adder, netserv, timeout, log, nOut, nIn)

		case "fetch":
			lg := log.With().Int(logging.Service, logging.FetchService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			s.servers[c.Type], s.fallbacks[c.Type] = fetch.NewServer(pid, dag, adder, netserv, timeout, log, nOut, nIn)

		default:
			return nil, nil, gomel.NewConfigError("unknown sync type: " + c.Type)
		}
	}

	for _, c := range configs {
		var service process.Service
		if c.Fallback != "" {
			fallback := s.fallbacks[c.Fallback]
			if c.Retry > 0 {
				lg := log.With().Int(logging.Service, logging.RetryingService).Logger()
				service, fallback = retrying.NewService(dag, adder, fallback, c.Retry, lg)
				s.subservices = append(s.subservices, service)
			}
			s.servers[c.Type].SetFallback(fallback)
		} else {
			s.servers[c.Type].SetFallback(sync.DefaultFallback(log))
		}
	}

	return s, func(unit gomel.Unit) {
		if s.mcServer != nil {
			s.mcServer.Send(unit)
		}
	}, nil
}

func (s *service) Start() error {
	for _, service := range s.subservices {
		service.Start()
	}
	for _, server := range s.servers {
		server.Start()
	}
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	for _, server := range s.servers {
		server.StopOut()
	}
	// let other processes sync with us some more
	time.Sleep(5 * time.Second)
	for _, server := range s.servers {
		server.StopIn()
	}
	for _, service := range s.subservices {
		service.Stop()
	}
	s.log.Info().Msg(logging.ServiceStopped)
}

// Checks if the list of configs is valid that is:
// a) every server defined as fallback is present
// b) there is only one multicasting server
func valid(configs []*process.Sync) error {
	if len(configs) == 0 {
		return gomel.NewConfigError("empty sync configuration")
	}
	availFbks := map[string]bool{}
	mc := false
	for _, c := range configs {
		if c.Type == "fetch" || c.Type == "gossip" || c.Type == "retrying" {
			availFbks[c.Type] = true
		}
		if c.Type == "multicast" || c.Type == "rmc" {
			if mc {
				return gomel.NewConfigError("multiple multicast servers defined")
			}
			mc = true
		}
	}
	for _, c := range configs {
		if c.Fallback == "" {
			continue
		}
		if !availFbks[c.Fallback] {
			return gomel.NewConfigError("defined " + c.Fallback + " as fallback, but there is no configuration for it")
		}
	}
	return nil
}

// Return network.Server of the type indicated by "net". If needed, append a corresponding service to the given slice. Defaults to "tcp".
func getNetServ(net string, localAddress string, remoteAddresses []string, services []process.Service, log zerolog.Logger) (network.Server, []process.Service, error) {
	switch net {
	case "udp":
		netserv, err := udp.NewServer(localAddress, remoteAddresses, log)
		if err != nil {
			return nil, services, err
		}
		return netserv, services, nil
	case "pers":
		netserv, service, err := persistent.NewServer(localAddress, remoteAddresses, log)
		if err != nil {
			return nil, services, err
		}
		return netserv, append(services, service), nil
	default:
		netserv, err := tcp.NewServer(localAddress, remoteAddresses, log)
		if err != nil {
			return nil, services, err
		}
		return netserv, services, nil
	}
}
