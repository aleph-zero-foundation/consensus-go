package syncer

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/config"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	lg "gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/fetch"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
	"gitlab.com/alephledger/consensus-go/pkg/sync/rmc"
	"gitlab.com/alephledger/core-go/pkg/core"
	"gitlab.com/alephledger/core-go/pkg/network"
	"gitlab.com/alephledger/core-go/pkg/network/persistent"
	"gitlab.com/alephledger/core-go/pkg/network/tcp"
	"gitlab.com/alephledger/core-go/pkg/network/udp"
)

type syncer struct {
	gossip      sync.Gossip
	fetch       sync.Fetch
	mcast       sync.Multicast
	servers     []core.Service
	subservices []core.Service
}

// New creates a new syncer that uses provided config, ordered and logger.
func New(conf config.Config, orderer gomel.Orderer, log zerolog.Logger, setup bool) (gomel.Syncer, error) {
	s := &syncer{}

	// init fetch
	var netserv network.Server
	var err error
	netserv, s.subservices, err = getNetServ(conf.FetchNetType, conf.Pid, conf.FetchAddresses, s.subservices, conf.Timeout, log)
	if err != nil {
		return nil, err
	}
	serv, ftrigger := fetch.NewServer(conf, orderer, netserv, log.With().Int(lg.Service, lg.FetchService).Logger())
	s.servers = append(s.servers, serv)
	s.fetch = ftrigger
	// init gossip
	netserv, s.subservices, err = getNetServ(conf.GossipNetType, conf.Pid, conf.GossipAddresses, s.subservices, conf.Timeout, log)
	if err != nil {
		return nil, err
	}
	serv, gtrigger := gossip.NewServer(conf, orderer, netserv, log.With().Int(lg.Service, lg.GossipService).Logger())
	s.servers = append(s.servers, serv)
	s.gossip = gtrigger
	if setup {
		// init rmc
		netserv, s.subservices, err = getNetServ(conf.RMCNetType, conf.Pid, conf.RMCAddresses, s.subservices, conf.Timeout, log)
		if err != nil {
			return nil, err
		}
		serv, s.mcast = rmc.NewServer(conf, orderer, netserv, log.With().Int(lg.Service, lg.RMCService).Logger())
		s.servers = append(s.servers, serv)
	} else {
		// init mcast
		netserv, s.subservices, err = getNetServ(conf.MCastNetType, conf.Pid, conf.MCastAddresses, s.subservices, conf.Timeout, log)
		if err != nil {
			return nil, err
		}
		serv, s.mcast = multicast.NewServer(conf, orderer, netserv, log.With().Int(lg.Service, lg.MCService).Logger())
		s.servers = append(s.servers, serv)
	}
	return s, nil
}

func (s *syncer) Multicast(u gomel.Unit)                { s.mcast(u) }
func (s *syncer) RequestFetch(pid uint16, ids []uint64) { s.fetch(pid, ids) }
func (s *syncer) RequestGossip(pid uint16)              { s.gossip(pid) }

func (s *syncer) Start() {
	for _, service := range s.subservices {
		service.Start()
	}
	for _, server := range s.servers {
		server.Start()
	}
}

func (s *syncer) Stop() {
	for _, service := range s.subservices {
		service.Stop()
	}
	for _, server := range s.servers {
		server.Stop()
	}
}

type networkService struct {
	wrapped network.Server
}

func newNetworkService(netserv network.Server) core.Service {
	return &networkService{wrapped: netserv}
}

func (ns *networkService) Start() error {
	return nil
}

func (ns *networkService) Stop() {
	ns.wrapped.Stop()
}

// Return network.Server of the type indicated by "net". If needed, append a corresponding service to the given slice. Defaults to "tcp".
func getNetServ(net string, pid uint16, addresses []string, services []core.Service, timeout time.Duration, log zerolog.Logger) (network.Server, []core.Service, error) {
	switch net {
	case "udp":
		netLogger := log.With().Int(lg.Service, lg.NetworkService).Logger()
		netserv, err := udp.NewServer(addresses[pid], addresses, netLogger)
		if err != nil {
			return nil, services, err
		}
		netService := newNetworkService(netserv)
		services = append(services, netService)

		return netserv, services, nil
	case "pers":
		netLogger := log.With().Int(lg.Service, lg.NetworkService).Logger()
		netserv, err := tcp.NewServer(addresses[pid], addresses, netLogger)
		if err != nil {
			return nil, services, err
		}
		netserv = network.NewTimeoutConnectionServer(netserv, timeout)
		netService := newNetworkService(netserv)
		services = append(services, netService)

		var service core.Service

		netserv, service, err = persistent.NewServer(addresses[pid], addresses, timeout)
		if err != nil {
			return nil, services, err
		}
		services = append(services, service)

		return netserv, services, nil
	default:
		netLogger := log.With().Int(lg.Service, lg.NetworkService).Logger()
		netserv, err := tcp.NewServer(addresses[pid], addresses, netLogger)
		if err != nil {
			return nil, services, err
		}
		netserv = network.NewTimeoutConnectionServer(netserv, timeout)
		netService := newNetworkService(netserv)
		services = append(services, netService)

		return netserv, services, nil
	}
}
