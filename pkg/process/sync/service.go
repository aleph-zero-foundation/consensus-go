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
	log             zerolog.Logger
}

// NewService creates a new syncing service for the given dag, with the given config.
func NewService(dag gomel.Dag, randomSource gomel.RandomSource, config *process.Sync, mcRequests <-chan multicast.MCRequest, attemptTiming chan<- int, log zerolog.Logger) (process.Service, error) {
	nProc := uint16(dag.NProc())
	pid := uint16(config.Pid)
	dialer, listener, err := tcp.NewNetwork(config.LocalAddress, config.RemoteAddresses, log)
	if err != nil {
		return nil, err
	}
	peerSource := gossip.NewDefaultPeerSource(nProc, pid)
	gossipProto := gossip.NewProtocol(pid, dag, randomSource, dialer, listener, peerSource, config.Timeout, attemptTiming, log)

	var dialerMC network.Dialer
	var listenerMC network.Listener
	if config.UDPMulticast {
		dialerMC, listenerMC, err = udp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, log)
	} else {
		dialerMC, listenerMC, err = tcp.NewNetwork(config.LocalMCAddress, config.RemoteMCAddresses, log)
	}
	if err != nil {
		return nil, err
	}
	multicastProto := multicast.NewProtocol(pid, dag, randomSource, dialerMC, listenerMC, config.Timeout, mcRequests, log)

	return &service{
		gossipServer:    sync.NewServer(gossipProto, config.OutSyncLimit, config.InSyncLimit, log),
		multicastServer: sync.NewServer(multicastProto, 4*uint(nProc), 2*uint(nProc), log),
		log:             log,
	}, nil
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
	// let other processes sync with us some more
	time.Sleep(5 * time.Second)
	s.gossipServer.StopIn()
	s.multicastServer.StopIn()
	s.log.Info().Msg(logging.ServiceStopped)
}
