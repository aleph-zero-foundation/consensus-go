package sync

import (
	"time"

	"github.com/rs/zerolog"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/network/udp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
)

type service struct {
	servers []sync.Server
	log     zerolog.Logger
}

// NewService creates a new syncing service for the given dag, with the given config. Returns also a callback meant to be called after creating a new unit.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Sync, primeAlert gomel.Callback, log zerolog.Logger) (process.Service, gomel.Callback, error) {
	pid := uint16(config.Pid)
	nProc := uint16(dag.NProc())
	s := &service{log: log.With().Int(logging.Service, logging.SyncService).Logger()}
	gossipLog := log.With().Int(logging.Service, logging.GossipService).Logger()
	mcLog := log.With().Int(logging.Service, logging.MCService).Logger()
	callback := gomel.NopCallback()

	dialer, listener, err := tcp.NewNetwork(config.LocalAddress, config.RemoteAddresses, gossipLog)
	if err != nil {
		return nil, nil, err
	}
	peerSource := gossip.NewDefaultPeerSource(nProc, pid)
	gossipProto := gossip.NewProtocol(pid, dag, randomSource, dialer, listener, peerSource, primeAlert, config.Timeout, gossipLog)
	gossipServer := sync.NewDefaultServer(gossipProto, config.OutSyncLimit, config.InSyncLimit)
	s.servers = append(s.servers, gossipServer)

	switch config.Multicast {
	case "tcp":
		dialer, listener, err = tcp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, mcLog)
	case "udp":
		dialer, listener, err = udp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, mcLog)
	default:
		return s, callback, nil
	}
	if err != nil {
		return nil, nil, err
	}
	multicastServer, callback := multicast.NewServer(pid, dag, randomSource, dialer, listener, primeAlert, config.Timeout, sync.NopFallback(), mcLog)
	s.servers = append(s.servers, multicastServer)

	return s, callback, nil
}

func (s *service) Start() error {
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
	s.log.Info().Msg(logging.ServiceStopped)
}
