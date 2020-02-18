// Package sync implements a service that creates and runs all the necessary syncing servers.
package sync

import (
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
	"gitlab.com/alephledger/consensus-go/pkg/sync/rmc"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/network/persistent"
	"gitlab.com/alephledger/core-go/pkg/network/tcp"
	"gitlab.com/alephledger/core-go/pkg/network/udp"
	rmcbox "gitlab.com/alephledger/core-go/pkg/rmc"
)

type syncer struct {
	gossip      sync.Gossip
	fetch       sync.Fetch
	mCast       sync.Multicast
	servers     []sync.Server
	subservices []gomel.Service
	log         zerolog.Logger
}

var logNames = map[string]int{
	"multicast": logging.MCService,
	"rmc":       logging.RMCService,
	"gossip":    logging.GossipService,
	"fetch":     logging.FetchService,
}

// New creates a new syncer.
// Each config entry corresponds to a separate sync.Server.
// The returned function should be called on units created by this process after they are added to the poset.
func New(conf config.Config, orderer gomel.Orderer, log zerolog.Logger) (gomel.Syncer, error) {
	err, configs := valid(conf)
	if err != nil {
		return nil, err
	}
	s := &service{
		servers: make([]sync.Server, len(configs)),
		log:     log.With().Int(logging.Service, logging.SyncService).Logger(),
	}

	var netserv network.Server
	for i, c := range configs {

		timeout, err := conf.Timeout
		if err != nil {
			return nil, err
		}

		lg := log.With().Int(logging.Service, logNames[c.Type]).Logger()
		netserv, s.subservices, err = getNetServ(c.netType, c.addrs[conf.Pid], c.addrs, s.subservices)
		if err != nil {
			return nil, err
		}

		switch c.Type {
		case "multicast":
			s.servers[i], s.mcast = multicast.NewServer(conf, orderer, netserv, timeout, lg)

		case "rmc":
			s.servers[i], s.mcast = rmc.NewServer(conf, orderer, netserv, rmcbox.New(c.Pubs, c.Priv), timeout, lg)

		case "gossip":
			nOut, nIn, nIdle := c.workers[0], c.workers[1], c.workers[2]
			server, trigger := gossip.NewServer(conf, orderer, netserv, timeout, lg, nOut, nIn, nIdle)
			s.servers[i] = server
			s.reqGossip = trigger

		case "fetch":
			nOut, nIn := c.workers[0], c.workers[1]
			server, trigger := fetch.NewServer(conf, orderer, netserv, timeout, lg, nOut, nIn)
			s.servers[i] = server
			s.reqFetch = trigger

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

type syncConf struct {
	netType string
	addrs   []string
	workers []int
}

// Checks if the list of configs is valid, that means there is only one multicasting server.
func valid(conf config.Config) ([]syncConf, error) {
	scs := []syncConf{}

	// parse rmc configuration
	if len(conf.RMCAddresses) != int(conf.NProc) {
		return nil, gomel.NewConfigError("wrong number of rmc addresses")
	}
	scs = append(scs, syncConf{conf.RMCNetType, conf.RMCAddresses})
	// parse mcast configuration
	if len(conf.MCastAddresses) == int(conf.NProc) {
		// we use only one type of multicast, so we drop rmc conf and use it for alerts
		scs = syncConf{conf.MCastNetType, conf.MCastAddresses}
	}
	// parse gossip configuration
	if len(conf.GossipAddresses) == int(conf.NProc) {
		scs = append(scs, syncConf{conf.GossipNetType, conf.GossipAddresses, conf.GossipWorkers})
	}
	// parse fetch configuration
	if len(conf.FetchAddresses) == int(conf.NProc) {
		scs = append(syncConf{conf.FetchNetType, conf.FetchAddresses, conf.FetchWorkers})
	}

	return scs, nil
}

// Return network.Server of the type indicated by "net". If needed, append a corresponding service to the given slice. Defaults to "tcp".
func getNetServ(net string, localAddress string, remoteAddresses []string, services []gomel.Service) (network.Server, []gomel.Service, error) {
	switch net {
	case "udp":
		netserv, err := udp.NewServer(localAddress, remoteAddresses)
		if err != nil {
			return nil, services, err
		}
		return netserv, services, nil
	case "pers":
		netserv, service, err := persistent.NewServer(localAddress, remoteAddresses)
		if err != nil {
			return nil, services, err
		}
		return netserv, append(services, service), nil
	default:
		netserv, err := tcp.NewServer(localAddress, remoteAddresses)
		if err != nil {
			return nil, services, err
		}
		return netserv, services, nil
	}
}
