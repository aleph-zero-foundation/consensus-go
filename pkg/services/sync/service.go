// Package sync implements a service that creates and runs all the necessary syncing servers.
package sync

import (
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
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
	servers     []sync.Server
	subservices []gomel.Service
	log         zerolog.Logger
}

// NewService creates a new syncing service and the function for multicasting units.
// Each config entry corresponds to a separate sync.Server.
// The returned function should be called on units created by this process after they are added to the poset.
func NewService(dag gomel.Dag, adder gomel.Adder, configs []*config.Sync, log zerolog.Logger) (gomel.Service, error) {
	if err := valid(configs); err != nil {
		return nil, err
	}
	pid := configs[0].Pid
	s := &service{
		servers: make([]sync.Server, len(configs)),
		log:     log.With().Int(logging.Service, logging.SyncService).Logger(),
	}

	logNames := map[string]int{
		"multicast": logging.MCService,
		"rmc":       logging.RMCService,
		"gossip":    logging.GossipService,
		"fetch":     logging.FetchService,
	}

	var netserv network.Server
	for i, c := range configs {

		timeout, err := time.ParseDuration(c.Params["timeout"])
		if err != nil {
			return nil, err
		}

		lg := log.With().Int(logging.Service, logNames[c.Type]).Logger()
		netserv, s.subservices, err = getNetServ(c.Params["network"], c.LocalAddress, c.RemoteAddresses, s.subservices, lg)
		if err != nil {
			return nil, err
		}

		switch c.Type {
		case "multicast":
			s.servers[i] = multicast.NewServer(pid, dag, adder, netserv, timeout, lg)

		case "rmc":
			s.servers[i] = rmc.NewServer(pid, dag, adder, netserv, rmcbox.New(c.Pubs, c.Priv), timeout, lg)

		case "gossip":
			nIn, nOut, err := parseInOut(c.Params)
			if err != nil {
				return nil, err
			}
			s.servers[i] = gossip.NewServer(pid, dag, adder, netserv, timeout, lg, nOut, nIn)

		case "fetch":
			nIn, nOut, err := parseInOut(c.Params)
			if err != nil {
				return nil, err
			}
			s.servers[i] = fetch.NewServer(pid, dag, adder, netserv, timeout, lg, nOut, nIn)

		default:
			return nil, gomel.NewConfigError("unknown sync type: " + c.Type)
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

// Checks if the list of configs is valid, that means there is only one multicasting server.
func valid(configs []*config.Sync) error {
	if len(configs) == 0 {
		return gomel.NewConfigError("empty sync configuration")
	}
	mc := false
	for i, c := range configs {
		if c.Type == "multicast" || c.Type == "rmc" {
			if mc {
				return gomel.NewConfigError("multiple multicast servers defined")
			}
			mc = true
			if i != 0 {
				return gomel.NewConfigError("multicast sync servers need to be defined before any other servers")
			}
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

func parseInOut(params map[string]string) (int, int, error) {
	nOut, err := strconv.Atoi(params["nOut"])
	if err != nil {
		return 0, 0, err
	}
	nIn, err := strconv.Atoi(params["nIn"])
	if err != nil {
		return 0, 0, err
	}
	return nIn, nOut, nil
}
