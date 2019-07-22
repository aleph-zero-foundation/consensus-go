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

const (
	// Some magic numbers for multicast. All below are ratios, they get multiplied with nProc.
	mcRequestsSize = 10
	mcOutWPSize    = 4
	mcInWPSize     = 2
)

type service struct {
	gossipServer    *sync.Server
	multicastServer *sync.Server
	mcRequests      chan multicast.MCRequest
	log             zerolog.Logger
}

// NewService creates a new syncing service for the given dag, with the given config.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Sync, attemptTiming chan<- int, log zerolog.Logger) (process.Service, func(gomel.Unit), error) {
	nProc := uint16(dag.NProc())
	pid := uint16(config.Pid)
	syncLog := log.With().Int(logging.Service, logging.SyncService).Logger()
	gossipLog := log.With().Int(logging.Service, logging.GossipService).Logger()
	mcLog := log.With().Int(logging.Service, logging.MCService).Logger()
	var dialerMC network.Dialer
	var listenerMC network.Listener

	dialer, listener, err := tcp.NewNetwork(config.LocalAddress, config.RemoteAddresses, gossipLog)
	if err != nil {
		return nil, nil, err
	}
	peerSource := gossip.NewDefaultPeerSource(nProc, pid)
	gossipProto := gossip.NewProtocol(pid, dag, randomSource, dialer, listener, peerSource, config.Timeout, attemptTiming, gossipLog)

	switch config.Multicast {
	case "tcp":
		dialerMC, listenerMC, err = tcp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, mcLog)
		if err != nil {
			return nil, nil, err
		}
	case "udp":
		dialerMC, listenerMC, err = udp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, mcLog)
		if err != nil {
			return nil, nil, err
		}
	default:
		return &service{
				gossipServer:    sync.NewServer(gossipProto, config.OutSyncLimit, config.InSyncLimit),
				multicastServer: sync.NopServer(),
				mcRequests:      nil,
				log:             syncLog,
			},
			func(unit gomel.Unit) {}, nil
	}

	mcRequests := make(chan multicast.MCRequest, mcRequestsSize*nProc)
	multicastProto := multicast.NewProtocol(pid, dag, randomSource, dialerMC, listenerMC, config.Timeout, mcRequests, mcLog)

	return &service{
			gossipServer:    sync.NewServer(gossipProto, config.OutSyncLimit, config.InSyncLimit),
			multicastServer: sync.NewServer(multicastProto, uint(mcOutWPSize*nProc), uint(mcOutWPSize*nProc)),
			mcRequests:      mcRequests,
			log:             syncLog,
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
	close(s.mcRequests)
	s.gossipServer.StopOut()
	s.multicastServer.StopOut()
	// let other processes sync with us some more
	time.Sleep(5 * time.Second)
	s.gossipServer.StopIn()
	s.multicastServer.StopIn()
	s.log.Info().Msg(logging.ServiceStopped)
}
