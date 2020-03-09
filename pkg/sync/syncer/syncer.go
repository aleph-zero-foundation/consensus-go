package syncer

import (
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
	servers     []sync.Server
	subservices []core.Service
}

// New creates a new syncer that uses provided config, ordered and logger.
func New(conf config.Config, orderer gomel.Orderer, log zerolog.Logger) (gomel.Syncer, error) {
	err := valid(conf)
	if err != nil {
		return nil, err
	}
	s := &syncer{}

	var serv sync.Server
	var netserv network.Server
	if len(conf.RMCAddresses) == int(conf.NProc) && len(conf.MCastAddresses) == 0 {
		netserv, s.subservices, err = getNetServ(conf.RMCNetType, conf.Pid, conf.RMCAddresses, s.subservices)
		if err != nil {
			return nil, err
		}
		serv, s.mcast = rmc.NewServer(conf, orderer, netserv, log.With().Int(logging.Service, logging.RMCService).Logger())
		s.servers = append(s.servers, serv)
	}
	if len(conf.MCastAddresses) == int(conf.NProc) {
		netserv, s.subservices, err = getNetServ(conf.MCastNetType, conf.Pid, conf.MCastAddresses, s.subservices)
		if err != nil {
			return nil, err
		}
		serv, s.mcast = multicast.NewServer(conf, orderer, netserv, log.With().Int(logging.Service, logging.MCService).Logger())
		s.servers = append(s.servers, serv)
	}
	if len(conf.FetchAddresses) == int(conf.NProc) {
		netserv, s.subservices, err = getNetServ(conf.FetchNetType, conf.Pid, conf.FetchAddresses, s.subservices)
		if err != nil {
			return nil, err
		}
		serv, trigger := fetch.NewServer(conf, orderer, netserv, log.With().Int(logging.Service, logging.FetchService).Logger())
		s.servers = append(s.servers, serv)
		s.fetch = trigger
	}
	if len(conf.GossipAddresses) == int(conf.NProc) {
		netserv, s.subservices, err = getNetServ(conf.GossipNetType, conf.Pid, conf.GossipAddresses, s.subservices)
		if err != nil {
			return nil, err
		}
		serv, trigger := gossip.NewServer(conf, orderer, netserv, log.With().Int(logging.Service, logging.GossipService).Logger())
		s.servers = append(s.servers, serv)
		s.gossip = trigger
	}
	if s.gossip == nil {
		s.gossip = func(uint16) {}
		log.Info().Str("where", "syncer").Msg("using `noop` gossip")
	}
	if s.fetch == nil {
		s.fetch = func(uint16, []uint64) {}
		log.Info().Str("where", "syncer").Msg("using `noop` fetch")
	}
	if s.mcast == nil {
		s.mcast = func(gomel.Unit) {}
		log.Info().Str("where", "syncer").Msg("using `noop` mcast")
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
}

// Checks if the config entries for syncer are is valid:
// the number of addresses for each server must be NProc or 0
func valid(conf config.Config) error {
	ok := func(i int) bool { return i == int(conf.NProc) || i == 0 }

	if !ok(len(conf.RMCAddresses)) {
		return gomel.NewConfigError("syncer: wrong number of rmc addresses")
	}
	if !ok(len(conf.MCastAddresses)) {
		return gomel.NewConfigError("syncer: wrong number of multicast addresses")
	}
	if !ok(len(conf.GossipAddresses)) {
		return gomel.NewConfigError("syncer: wrong number of gossip addresses")
	}
	if !ok(len(conf.FetchAddresses)) {
		return gomel.NewConfigError("syncer: wrong number of fetch addresses")
	}

	rmcOn := len(conf.RMCAddresses) == int(conf.NProc)
	mcOn := len(conf.MCastAddresses) == int(conf.NProc)
	if !mcOn && !rmcOn {
		return gomel.NewConfigError("syncer: both RMC and multicast disabled")
	}
	return nil
}

// Return network.Server of the type indicated by "net". If needed, append a corresponding service to the given slice. Defaults to "tcp".
func getNetServ(net string, pid uint16, addresses []string, services []core.Service) (network.Server, []core.Service, error) {
	switch net {
	case "udp":
		netserv, err := udp.NewServer(addresses[pid], addresses)
		if err != nil {
			return nil, services, err
		}
		return netserv, services, nil
	case "pers":
		netserv, service, err := persistent.NewServer(addresses[pid], addresses)
		if err != nil {
			return nil, services, err
		}
		return netserv, append(services, service), nil
	default:
		netserv, err := tcp.NewServer(addresses[pid], addresses)
		if err != nil {
			return nil, services, err
		}
		return netserv, services, nil
	}
}
