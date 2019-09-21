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
	queryServers map[string]sync.QueryServer
	mcServer     sync.MulticastServer
	subservices  []process.Service
	log          zerolog.Logger
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
		queryServers: make(map[string]sync.QueryServer),
		log:          log.With().Int(logging.Service, logging.SyncService).Logger(),
	}

	servmap := make(map[string]sync.Server)

	for _, c := range configs {
		var netserv network.Server

		timeout, err := time.ParseDuration(c.Params["timeout"])
		if err != nil {
			return nil, nil, err
		}

		switch c.Type {
		case "multicast":
			log = log.With().Int(logging.Service, logging.MCService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, log)
			server := multicast.NewServer(pid, dag, adder, netserv, timeout, log)
			s.mcServer = server
			servmap[c.Type] = server

		case "rmc":
			log = log.With().Int(logging.Service, logging.RMCService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, log)
			server := rmc.NewServer(pid, dag, adder, netserv, rmcbox.New(c.Pubs, c.Priv), timeout, log)
			s.mcServer = server
			servmap[c.Type] = server

		case "gossip":
			log = log.With().Int(logging.Service, logging.GossipService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, log)
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			server := gossip.NewServer(pid, dag, adder, netserv, timeout, log, nOut, nIn)
			s.queryServers[c.Type] = server
			servmap[c.Type] = server

		case "fetch":
			log = log.With().Int(logging.Service, logging.FetchService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, log)
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, nil, err
			}
			server := fetch.NewServer(pid, dag, adder, netserv, timeout, log, nOut, nIn)
			s.queryServers[c.Type] = server
			servmap[c.Type] = server

		case "retrying":
			log := log.With().Int(logging.Service, logging.RetryingService).Logger()
			interval, err := time.ParseDuration(c.Params["interval"])
			if err != nil {
				return nil, nil, err
			}
			server := retrying.NewServer(dag, adder, interval, log)
			s.queryServers[c.Type] = server
			servmap[c.Type] = server

		default:
			return nil, nil, gomel.NewConfigError("unknown sync type: " + c.Type)
		}
	}

	for _, c := range configs {
		if c.Fallback != "" {
			servmap[c.Type].SetFallback(s.queryServers[c.Fallback])
		}
	}

	return s, func(unit gomel.Unit) { s.mcServer.Send(unit) }, nil
}

func (s *service) Start() error {
	for _, service := range s.subservices {
		service.Start()
	}
	for _, server := range s.queryServers {
		server.Start()
	}
	s.mcServer.Start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.mcServer.StopOut()
	for _, server := range s.queryServers {
		server.StopOut()
	}
	// let other processes sync with us some more
	time.Sleep(5 * time.Second)
	s.mcServer.StopIn()
	for _, server := range s.queryServers {
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
		netserv, err := persistent.NewServer(localAddress, remoteAddresses, log)
		if err != nil {
			return nil, services, err
		}
		return netserv, append(services, netserv.(process.Service)), nil
	default:
		netserv, err := tcp.NewServer(localAddress, remoteAddresses, log)
		if err != nil {
			return nil, services, err
		}
		return netserv, services, nil
	}
}
