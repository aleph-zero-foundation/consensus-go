// Package sync implements a service that creates and runs all the necessary syncing servers.
package sync

import (
	"errors"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	gdag "gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/dag/check"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/persistent"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/network/udp"
	rmcbox "gitlab.com/alephledger/consensus-go/pkg/rmc"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
	"gitlab.com/alephledger/consensus-go/pkg/sync/rmc"
)

type service struct {
	servers     map[string]sync.Server
	mcServer    sync.MulticastServer
	subservices []gomel.Service
	log         zerolog.Logger
}

var errMulticastFirst = errors.New("multicast sync servers need to be defined before any other servers")

// NewService creates a new syncing service and the function for multicasting units.
// Each config entry corresponds to a separate sync.Server.
// The returned function should be called on units created by this process after they are added to the poset.
func NewService(dag gomel.Dag, adder gomel.Adder, fetchData sync.FetchData, configs []*config.Sync, log zerolog.Logger) (gomel.Service, gomel.Dag, error) {
	if err := valid(configs); err != nil {
		return nil, nil, err
	}
	pid := configs[0].Pid
	fallbacks := make(map[string]sync.Fallback)
	s := &service{
		servers: make(map[string]sync.Server),
		log:     log.With().Int(logging.Service, logging.SyncService).Logger(),
	}

	for _, c := range configs {
		var netserv network.Server

		timeout, err := time.ParseDuration(c.Params["timeout"])
		if err != nil {
			return nil, nil, err
		}

		switch c.Type {
		case "multicast":
			if len(s.servers) > 0 {
				return nil, nil, errMulticastFirst
			}
			lg := log.With().Int(logging.Service, logging.MCService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			if err != nil {
				return nil, nil, err
			}
			server := multicast.NewServer(pid, dag, adder, netserv, timeout, lg)
			s.mcServer = server
			s.servers[c.Type] = server

		case "rmc":
			if len(s.servers) > 0 {
				return nil, nil, errMulticastFirst
			}
			lg := log.With().Int(logging.Service, logging.RMCService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			if err != nil {
				return nil, nil, err
			}
			server, fd, rmcCheck := rmc.NewServer(pid, dag, adder, netserv, rmcbox.New(c.Pubs, c.Priv), timeout, lg)
			if fetchData != nil {
				return nil, nil, errors.New("cannot use RMC with other services that require commitments")
			}
			fetchData = fd
			s.mcServer = server
			s.servers[c.Type] = server
			dag = check.AddCheck(dag, rmcCheck)

		case "gossip":
			lg := log.With().Int(logging.Service, logging.GossipService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			if err != nil {
				return nil, nil, err
			}
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			s.servers[c.Type], fallbacks[c.Type] = gossip.NewServer(pid, dag, adder, netserv, timeout, lg, nOut, nIn)
		case "fetch":
			lg := log.With().Int(logging.Service, logging.FetchService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
			if err != nil {
				return nil, nil, err
			}
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			s.servers[c.Type], fallbacks[c.Type] = fetch.NewServer(pid, dag, adder, netserv, timeout, lg, nOut, nIn)

		default:
			return nil, nil, gomel.NewConfigError("unknown sync type: " + c.Type)
		}
	}

	for _, c := range configs {
		if c.Fallback != "" {
			fallback := fallbacks[c.Fallback]
			s.servers[c.Type].SetFallback(fallback)
		} else {
			s.servers[c.Type].SetFallback(sync.DefaultFallback(log))
		}
		s.servers[c.Type].SetFetchData(fetchData)
	}

	return s, gdag.AfterInsert(dag, func(unit gomel.Unit) {
		if s.mcServer != nil && unit.Creator() == pid {
			s.mcServer.Send(unit)
		}
	}), nil
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
func valid(configs []*config.Sync) error {
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
func getNetServ(net string, localAddress string, remoteAddresses []string, services []gomel.Service, log zerolog.Logger) (network.Server, []gomel.Service, error) {
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
