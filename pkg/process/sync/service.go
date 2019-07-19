package sync

import (
	"time"

	"github.com/rs/zerolog"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/logging"
	"gitlab.com/alephledger/consensus-go/pkg/network"
	"gitlab.com/alephledger/consensus-go/pkg/network/tcp"
	"gitlab.com/alephledger/consensus-go/pkg/network/udp"
	"gitlab.com/alephledger/consensus-go/pkg/process"
	"gitlab.com/alephledger/consensus-go/pkg/sync"
	"gitlab.com/alephledger/consensus-go/pkg/sync/gossip"
	"gitlab.com/alephledger/consensus-go/pkg/sync/multicast"
)

type service struct {
	gossipServer    *sync.Server
	multicastServer *sync.Server
	mcRequests      chan multicast.MCRequest
	log             zerolog.Logger
}

// NewService creates a new syncing service for the given dag, with the given config.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Sync, mcRequests chan multicast.MCRequest, attemptTiming chan<- int, log zerolog.Logger) (process.Service, func(gomel.Unit), error) {
	nProc := uint16(dag.NProc())
	pid := uint16(config.Pid)
	gossipLog := log.With().Int(logging.Service, logging.GossipService).Logger()
	mcLog := log.With().Int(logging.Service, logging.MCService).Logger()
	dialer, listener, err := tcp.NewNetwork(config.LocalAddress, config.RemoteAddresses, gossipLog)
	if err != nil {
		return nil, nil, err
	}
	peerSource := gossip.NewDefaultPeerSource(nProc, pid)
	gossipProto := gossip.NewProtocol(pid, dag, randomSource, dialer, listener, peerSource, config.Timeout, attemptTiming, gossipLog)

	var dialerMC network.Dialer
	var listenerMC network.Listener
	if config.UDPMulticast {
		dialerMC, listenerMC, err = udp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, mcLog)
	} else {
		dialerMC, listenerMC, err = tcp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, mcLog)
	}
	if err != nil {
		return nil, nil, err
	}
	multicastProto := multicast.NewProtocol(pid, dag, randomSource, dialerMC, listenerMC, config.Timeout, mcRequests, mcLog)

	return &service{
			gossipServer:    sync.NewServer(gossipProto, config.OutSyncLimit, config.InSyncLimit),
			multicastServer: sync.NewServer(multicastProto, 4*uint(nProc), 2*uint(nProc)),
			mcRequests:      mcRequests,
			log:             log.With().Int(logging.Service, logging.SyncService).Logger(),
		},
		func(unit gomel.Unit) {
			err := multicast.Request(unit, mcRequests, pid, nProc)
			if err != nil {
				mcLog.Error().Str("where", "multicast.Request").Msg(err.Error())
			}
		},
		nil
}

func (s *service) Start() error {
	s.gossipServer.Start()
	s.multicastServer.Start()
	s.log.Info().Msg(logging.ServiceStarted)
	return nil
}

func (s *service) Stop() {
	s.gossipServer.StopOut()
	s.multicastServer.StopOut()
	close(s.mcRequests)
	// let other processes sync with us some more
	time.Sleep(5 * time.Second)
	s.gossipServer.StopIn()
	s.multicastServer.StopIn()
	s.log.Info().Msg(logging.ServiceStopped)
}
