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
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
	"gitlab.com/alephledger/consensus-go/pkg/sync/retrying"
)

type service struct {
	queryServers map[string]sync.QueryServer
	multicasters map[string]sync.MulticastServer
	servers      []sync.Server
	subservices  []process.Service
	log          zerolog.Logger
}

// NewService creates a new syncing service for the given dag, with the given config.
// When units received from a sync are added to the poset primeAlert is called on them.
// The returned callback should be called on units created by this process after they are added to the poset.
// It is used to multicast newly created units, when multicast is in use.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, configs []*process.Sync, log zerolog.Logger) (process.Service, error) {
	if err := valid(configs); err != nil {
		return nil, err
	}
	pid := configs[0].Pid
	s := &service{
		queryServers: make(map[string]sync.QueryServer),
		multicasters: make(map[string]sync.MulticastServer),
		log:          log.With().Int(logging.Service, logging.SyncService).Logger(),
	}

	servmap := make(map[string]sync.Server)

	for _, c := range configs {
		var (
			netserv network.Server
			server  sync.Server
		)
		tf, err := strconv.ParseFloat(c.Params["timeout"], 64)
		if err != nil {
			return nil, err
		}
		timeout := time.Duration(tf) * time.Second

		switch c.Type {
		case "multicast":
			log = log.With().Int(logging.Service, logging.MCService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, log)
			ms := multicast.NewServer(pid, dag, randomSource, netserv, timeout, log)
			s.multicasters[c.Type] = ms
			server = ms

		case "gossip":
			log = log.With().Int(logging.Service, logging.GossipService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, log)
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, err
			}
			qs := gossip.NewServer(pid, dag, randomSource, netserv, timeout, log, nOut, nIn)
			s.queryServers[c.Type] = qs
			server = qs

		case "fetch":
			log = log.With().Int(logging.Service, logging.FetchService).Logger()
			netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, log)
			nOut, err := strconv.Atoi(c.Params["nOut"])
			if err != nil {
				return nil, err
			}
			nIn, err := strconv.Atoi(c.Params["nIn"])
			if err != nil {
				return nil, err
			}
			qs := fetch.NewServer(pid, dag, randomSource, netserv, timeout, log, nOut, nIn)
			s.queryServers[c.Type] = qs
			server = qs

		case "retrying":
			log := log.With().Int(logging.Service, logging.RetryingService).Logger()
			rif, err := strconv.ParseFloat(c.Params["interval"], 64)
			if err != nil {
				return nil, err
			}
			interval := time.Millisecond * time.Duration(1000*rif)
			qs := retrying.NewServer(dag, randomSource, interval, log)
			s.queryServers[c.Type] = qs
			server = qs
		}
		s.servers = append(s.servers, server)
		servmap[c.Type] = server
	}

	for _, c := range configs {
		if c.Fallback != "" {
			servmap[c.Type].SetFallback(s.queryServers[c.Fallback])
		}
	}

	return s, nil
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
		server.SetFallback(nil)
	}

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

// Checks if all fallbacks have their corresponding configurations.
func valid(configs []*process.Sync) error {
	if len(configs) == 0 {
		return gomel.NewConfigError("empty sync configuration")
	}
	availFbks := map[string]bool{}
	for _, c := range configs {
		if c.Type == "fetch" || c.Type == "gossip" || c.Type == "retrying" {
			availFbks[c.Type] = true
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
